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

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/service/notify"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler/mock"
)

//go:generate mockgen -source=change_email.go -destination=mock/change_email.go -package=mock

type ChangeEmailSuite struct {
	suite.Suite

	ctrl        *gomock.Controller
	ctx         context.Context
	txManager   *mock.MockDBTxManager
	storage     *mock.MockuserEmailChanger
	revoker     *mock.MockoperationRevoker
	notifierAPI *mock.MockNotifier
	uc          *handler.ChangeEmail
}

func TestChangeEmailSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ChangeEmailSuite))
}

func (s *ChangeEmailSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.storage = mock.NewMockuserEmailChanger(s.ctrl)
	s.revoker = mock.NewMockoperationRevoker(s.ctrl)
	s.notifierAPI = mock.NewMockNotifier(s.ctrl)

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()

	s.uc = handler.NewChangeEmail(
		s.txManager,
		s.storage,
		s.revoker,
		s.notifierAPI,
		notify.NewActorProps(func(string) (string, string) {
			return "TestApp", "TestDevice"
		}),
	)
}

func (s *ChangeEmailSuite) payload() []byte {
	s.T().Helper()

	raw, err := unit.BuildChangeEmailPayload(dto.ChangeEmailOperation{NewEmail: "new@example.com", Email: "user@example.com"})
	s.Require().NoError(err)

	return raw
}

// TestExecute - email меняется на новый, незавершённые операции пользователя отзываются,
// уведомления о смене уходят на прежний и на новый адреса под разными ключами,
// оба - с контекстом клиента (время, IP, устройство).
func (s *ChangeEmailSuite) TestExecute() {
	userID := uuid.New()
	// адрес из заголовков прокси в уведомления не попадает - только реальный IP
	actor := dto.ActorMeta{
		UserID:   userID,
		ClientIP: mrtype.NewDetailedIP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("198.51.100.7")),
	}

	sent := make(map[string]map[string]any, 2)
	capture := func(_ context.Context, key string, props map[string]any) error {
		sent[key] = props

		return nil
	}

	gomock.InOrder(
		s.storage.EXPECT().UpdateEmail(gomock.Any(), userID, "new@example.com").Return(nil),
		s.revoker.EXPECT().RevokeAll(gomock.Any(), actor, logreason.EmailChanged).Return(nil),
		s.notifierAPI.EXPECT().Send(gomock.Any(), "user.email.changed", gomock.Any()).DoAndReturn(capture),
		s.notifierAPI.EXPECT().Send(gomock.Any(), "user.email.changed.new", gomock.Any()).DoAndReturn(capture),
	)

	s.Require().NoError(s.uc.Execute(s.ctx, actor, s.payload()))

	for key, to := range map[string]string{
		"user.email.changed":     "user@example.com",
		"user.email.changed.new": "new@example.com",
	} {
		props := sent[key]
		s.Equal(to, props["to"], key)
		s.Equal("user@example.com", props["oldEmail"], key)
		s.Equal("new@example.com", props["newEmail"], key)
		s.Equal("192.0.2.10", props["ip"], key)
		s.Equal("TestApp, TestDevice", props["device"], key)
		s.Contains(props, "occurredAt", key)
	}
}

// TestExecuteNotifyNewError - сбой постановки письма на новый адрес откатывает смену:
// ошибка возвращается, хотя письмо на прежний адрес уже поставлено в той же транзакции.
func (s *ChangeEmailSuite) TestExecuteNotifyNewError() {
	s.storage.EXPECT().UpdateEmail(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	s.revoker.EXPECT().RevokeAll(gomock.Any(), gomock.Any(), logreason.EmailChanged).Return(nil)
	gomock.InOrder(
		s.notifierAPI.EXPECT().Send(gomock.Any(), "user.email.changed", gomock.Any()).Return(nil),
		s.notifierAPI.EXPECT().
			Send(gomock.Any(), "user.email.changed.new", gomock.Any()).
			Return(errors.ErrInternalStorageQueryFailed.New()),
	)

	err := s.uc.Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, s.payload())
	s.Require().ErrorIs(err, errors.ErrInternalStorageQueryFailed)
}

// TestExecuteRevokeError - сбой отзыва операций откатывает смену: ошибка возвращается,
// уведомления о смене не отправляются.
func (s *ChangeEmailSuite) TestExecuteRevokeError() {
	s.storage.EXPECT().UpdateEmail(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	s.revoker.EXPECT().
		RevokeAll(gomock.Any(), gomock.Any(), logreason.EmailChanged).
		Return(errors.ErrInternalStorageQueryFailed.New())

	err := s.uc.Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, s.payload())
	s.Require().ErrorIs(err, errors.ErrInternalStorageQueryFailed)
}

// TestExecuteEmailTaken - новый адрес успели занять за время жизни операции: нарушение
// уникальности становится пользовательской ошибкой EmailAlreadyExists (400), а не 500,
// операции не отзываются и уведомления о смене не отправляются (моки без EXPECT: любой
// вызов провалит тест).
func (s *ChangeEmailSuite) TestExecuteEmailTaken() {
	s.storage.EXPECT().
		UpdateEmail(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.ErrInternalStorageDuplicateKeyViolation.New())

	err := s.uc.Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, s.payload())
	s.Require().ErrorIs(err, mrauth.ErrEmailAlreadyExists)
}

// TestExecuteStorageError - прочие сбои хранилища остаются внутренними, а не выдаются
// за занятый адрес.
func (s *ChangeEmailSuite) TestExecuteStorageError() {
	s.storage.EXPECT().
		UpdateEmail(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.ErrInternalStorageQueryFailed.New())

	err := s.uc.Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, s.payload())
	s.Require().Error(err)
	s.Require().NotErrorIs(err, mrauth.ErrEmailAlreadyExists)
}
