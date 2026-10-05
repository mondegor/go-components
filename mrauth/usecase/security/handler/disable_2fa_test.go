package handler_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/service/notify"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler/mock"
)

//go:generate mockgen -source=disable_2fa.go -destination=mock/disable_2fa.go -package=mock

type Disable2FASuite struct {
	suite.Suite

	ctrl        *gomock.Controller
	ctx         context.Context
	txManager   *mock.MockDBTxManager
	storage     *mock.Mockuser2faDisabler
	revoker     *mock.MockoperationRevoker
	notifierAPI *mock.MockNotifier
	securityLog *mock.MocksecurityLogStorage
	uc          *handler.Disable2FA
}

func TestDisable2FASuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(Disable2FASuite))
}

func (s *Disable2FASuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.storage = mock.NewMockuser2faDisabler(s.ctrl)
	s.revoker = mock.NewMockoperationRevoker(s.ctrl)
	s.notifierAPI = mock.NewMockNotifier(s.ctrl)
	s.securityLog = mock.NewMocksecurityLogStorage(s.ctrl)

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()

	s.uc = handler.NewDisable2FA(s.txManager, s.storage, s.revoker, s.notifierAPI, notify.NewActorProps(nil), s.securityLog)
}

func (s *Disable2FASuite) payload() []byte {
	s.T().Helper()

	raw, err := unit.BuildDisable2FAPayload(dto.Disable2FAOperation{Email: "user@example.com"})
	s.Require().NoError(err)

	return raw
}

// 2FA отключается, все незавершённые операции пользователя отзываются (их цепочки построены
// при включённой 2FA), снятие записывается в журнал безопасности, пользователю уходит
// уведомление с контекстом клиента.
func (s *Disable2FASuite) TestExecute() {
	userID := uuid.New()
	actor := dto.ActorMeta{UserID: userID}

	s.storage.EXPECT().Delete(gomock.Any(), userID).Return(nil)
	s.revoker.EXPECT().RevokeAll(gomock.Any(), actor, logreason.Auth2FAStateChanged).Return(nil)
	s.securityLog.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, row entity.SecurityLogEvent) error {
			s.Equal(userID, row.UserID)
			s.Equal(securityevent.Auth2FADisabled, row.EventType)
			s.Nil(row.Extra)

			return nil
		},
	)
	s.notifierAPI.EXPECT().
		Send(gomock.Any(), "user.2fa.disabled", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, props map[string]any) error {
			s.Equal("user@example.com", props["to"])
			s.Contains(props, "occurredAt")
			s.Contains(props, "ip")
			s.Contains(props, "device")

			return nil
		})

	s.Require().NoError(s.uc.Execute(s.ctx, actor, s.payload()))
}

// ошибка отзыва операций отменяет применение: уведомление не отправляется
// (мок Send без EXPECT: любой вызов провалит тест), транзакция откатывается целиком.
func (s *Disable2FASuite) TestExecuteRevokeError() {
	userID := uuid.New()

	s.storage.EXPECT().Delete(gomock.Any(), userID).Return(nil)
	s.revoker.EXPECT().RevokeAll(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("storage is down"))

	s.Require().Error(s.uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, s.payload()))
}

// ошибка записи в журнал безопасности отменяет применение: уведомление не отправляется
// (мок Send без EXPECT: любой вызов провалит тест), транзакция откатывается целиком.
func (s *Disable2FASuite) TestExecuteSecurityLogError() {
	userID := uuid.New()

	s.storage.EXPECT().Delete(gomock.Any(), userID).Return(nil)
	s.revoker.EXPECT().RevokeAll(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	s.securityLog.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(errors.New("storage is down"))

	s.Require().Error(s.uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, s.payload()))
}

// повторное применение подтверждённой операции застаёт 2FA уже отключённой: отсутствие
// записи не является ошибкой, но и уведомление повторно не отправляется - оно ушло при
// первом применении. Операции не отзываются: 2FA снята раньше, и операции, начатые после
// этого, построены при текущем состоянии и действуют (моки Send и RevokeAll без EXPECT).
func (s *Disable2FASuite) TestExecuteAlreadyDisabled() {
	userID := uuid.New()

	s.storage.EXPECT().Delete(gomock.Any(), userID).Return(errors.ErrEventStorageNoRecordFound)

	s.Require().NoError(s.uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, s.payload()))
}

// владелец операции известен на момент её применения,
// поэтому пустой userID - ошибка проводки (мок Delete без EXPECT: любой вызов провалит тест).
func (s *Disable2FASuite) TestExecuteEmptyUserID() {
	err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.payload())
	s.Require().ErrorIs(err, errors.ErrInternalIncorrectInputData)
}

// прочие ошибки хранилища прозрачными не становятся.
func (s *Disable2FASuite) TestExecuteStorageError() {
	userID := uuid.New()
	errStorage := errors.New("storage is down")

	s.storage.EXPECT().Delete(gomock.Any(), userID).Return(errStorage)

	s.Require().Error(s.uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, s.payload()))
}
