package handler_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/service/notify"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler/mock"
)

//go:generate mockgen -source=change_phone.go -destination=mock/change_phone.go -package=mock

type ChangePhoneSuite struct {
	suite.Suite

	ctrl        *gomock.Controller
	ctx         context.Context
	txManager   *mock.MockDBTxManager
	storage     *mock.MockuserPhoneChanger
	notifierAPI *mock.MockNotifier
	securityLog *mock.MocksecurityLogStorage
	uc          *handler.ChangePhone
}

func TestChangePhoneSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ChangePhoneSuite))
}

func (s *ChangePhoneSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.storage = mock.NewMockuserPhoneChanger(s.ctrl)
	s.notifierAPI = mock.NewMockNotifier(s.ctrl)
	s.securityLog = mock.NewMocksecurityLogStorage(s.ctrl)

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()

	s.uc = handler.NewChangePhone(
		s.txManager,
		s.storage,
		s.notifierAPI,
		notify.NewActorProps(func(string) (string, string) {
			return "TestApp", "TestDevice"
		}),
		s.securityLog,
	)
}

func (s *ChangePhoneSuite) payload() []byte {
	s.T().Helper()

	raw, err := unit.BuildChangePhonePayload(dto.ChangePhoneOperation{NewPhone: 79991234567, Phone: 79001112233, Email: "user@example.com"})
	s.Require().NoError(err)

	return raw
}

// TestExecute - телефон меняется на новый, смена записывается в журнал безопасности с прежним
// и новым номером, уведомление о смене уходит на email пользователя с контекстом клиента
// (время, IP, устройство).
func (s *ChangePhoneSuite) TestExecute() {
	userID := uuid.New()
	actor := dto.ActorMeta{
		UserID:   userID,
		ClientIP: mrtype.NewIP(netip.MustParseAddr("192.0.2.10")),
	}

	gomock.InOrder(
		s.storage.EXPECT().UpdatePhone(gomock.Any(), userID, uint64(79991234567)).Return(nil),
		s.securityLog.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, row entity.SecurityLogEvent) error {
				s.Equal(userID, row.UserID)
				s.Equal(securityevent.PhoneChanged, row.EventType)
				s.Equal(&entity.SecurityLogExtra{OldValue: "+79001112233", NewValue: "+79991234567"}, row.Extra)

				return nil
			},
		),
		s.notifierAPI.EXPECT().
			Send(gomock.Any(), "user.phone.changed", gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, props map[string]any) error {
				s.Equal("user@example.com", props["to"])
				s.Equal("192.0.2.10", props["ip"])
				s.Equal("TestApp, TestDevice", props["device"])
				s.Contains(props, "occurredAt")

				return nil
			}),
	)

	s.Require().NoError(s.uc.Execute(s.ctx, actor, s.payload()))
}

// TestExecuteSecurityLogError - сбой записи в журнал безопасности откатывает смену:
// ошибка возвращается, уведомление не отправляется.
func (s *ChangePhoneSuite) TestExecuteSecurityLogError() {
	s.storage.EXPECT().UpdatePhone(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	s.securityLog.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(errors.ErrInternalStorageQueryFailed.New())

	err := s.uc.Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, s.payload())
	s.Require().ErrorIs(err, errors.ErrInternalStorageQueryFailed)
}

// TestExecuteEmptyUserID - владелец операции известен на момент её применения, поэтому
// пустой userID - ошибка проводки (мок UpdatePhone без EXPECT: любой вызов провалит тест).
func (s *ChangePhoneSuite) TestExecuteEmptyUserID() {
	err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.payload())
	s.Require().ErrorIs(err, errors.ErrInternalIncorrectInputData)
}

// TestExecuteBrokenPayload - нечитаемый payload операции не применяется (моки без EXPECT).
func (s *ChangePhoneSuite) TestExecuteBrokenPayload() {
	s.Require().Error(s.uc.Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, []byte(`{`)))
}

// TestExecuteStorageError - сбой смены телефона возвращается: журнал и уведомление не пишутся.
func (s *ChangePhoneSuite) TestExecuteStorageError() {
	s.storage.EXPECT().UpdatePhone(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.ErrInternalStorageQueryFailed.New())

	err := s.uc.Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, s.payload())
	s.Require().ErrorIs(err, errors.ErrInternalStorageQueryFailed)
}

// TestExecuteNotifyError - сбой постановки уведомления откатывает смену: ошибка возвращается.
func (s *ChangePhoneSuite) TestExecuteNotifyError() {
	s.storage.EXPECT().UpdatePhone(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	s.securityLog.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)
	s.notifierAPI.EXPECT().Send(gomock.Any(), "user.phone.changed", gomock.Any()).Return(errors.ErrInternalStorageQueryFailed.New())

	err := s.uc.Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, s.payload())
	s.Require().ErrorIs(err, errors.ErrInternalStorageQueryFailed)
}
