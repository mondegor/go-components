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
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler/mock"
)

//go:generate mockgen -source=disable_2fa.go -destination=mock/disable_2fa.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrnotifier.go -package=mock github.com/mondegor/go-components/mrnotifier NoteProducer

type Disable2FASuite struct {
	suite.Suite

	ctrl        *gomock.Controller
	ctx         context.Context
	txManager   *mock.MockDBTxManager
	storage     *mock.Mockuser2faDisabler
	notifierAPI *mock.MockNoteProducer
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
	s.notifierAPI = mock.NewMockNoteProducer(s.ctrl)

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()

	s.uc = handler.NewDisable2FA(s.txManager, s.storage, s.notifierAPI)
}

func (s *Disable2FASuite) payload() []byte {
	s.T().Helper()

	raw, err := unit.BuildDisable2FAPayload(dto.Disable2FAOperation{Email: "user@example.com"})
	s.Require().NoError(err)

	return raw
}

// 2FA отключается, пользователю уходит уведомление.
func (s *Disable2FASuite) TestExecute() {
	userID := uuid.New()

	s.storage.EXPECT().Delete(gomock.Any(), userID).Return(nil)
	s.notifierAPI.EXPECT().Send(gomock.Any(), "user.2fa.disabled", gomock.Any()).Return(nil)

	s.Require().NoError(s.uc.Execute(s.ctx, userID, s.payload()))
}

// повторное применение подтверждённой операции застаёт 2FA уже отключённой: отсутствие
// записи не является ошибкой, но и уведомление повторно не отправляется - оно ушло при
// первом применении (мок Send без EXPECT: любой вызов провалит тест).
func (s *Disable2FASuite) TestExecuteAlreadyDisabled() {
	userID := uuid.New()

	s.storage.EXPECT().Delete(gomock.Any(), userID).Return(errors.ErrEventStorageNoRecordFound)

	s.Require().NoError(s.uc.Execute(s.ctx, userID, s.payload()))
}

// прочие ошибки хранилища прозрачными не становятся.
func (s *Disable2FASuite) TestExecuteStorageError() {
	userID := uuid.New()
	errStorage := errors.New("storage is down")

	s.storage.EXPECT().Delete(gomock.Any(), userID).Return(errStorage)

	s.Require().Error(s.uc.Execute(s.ctx, userID, s.payload()))
}
