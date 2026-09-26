package handler_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
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
	notifierAPI *mock.MockNoteProducer
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
	s.notifierAPI = mock.NewMockNoteProducer(s.ctrl)

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()

	s.uc = handler.NewChangeEmail(s.txManager, s.storage, s.notifierAPI)
}

func (s *ChangeEmailSuite) payload() []byte {
	s.T().Helper()

	raw, err := unit.BuildChangeEmailPayload(dto.ChangeEmailOperation{NewEmail: "new@example.com", Email: "user@example.com"})
	s.Require().NoError(err)

	return raw
}

// TestExecute - email меняется на новый, уведомление о смене уходит на прежний адрес.
func (s *ChangeEmailSuite) TestExecute() {
	userID := uuid.New()

	s.storage.EXPECT().UpdateEmail(gomock.Any(), userID, "new@example.com").Return(nil)
	s.notifierAPI.EXPECT().
		Send(gomock.Any(), "user.email.changed", map[string]any{"to": "user@example.com"}).
		Return(nil)

	s.Require().NoError(s.uc.Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, s.payload()))
}

// TestExecuteEmailTaken - новый адрес успели занять за время жизни операции: нарушение
// уникальности становится пользовательской ошибкой EmailAlreadyExists (400), а не 500,
// и уведомление о смене не отправляется (мок Send без EXPECT: любой вызов провалит тест).
func (s *ChangeEmailSuite) TestExecuteEmailTaken() {
	s.storage.EXPECT().
		UpdateEmail(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.ErrInternalStorageDuplicateKeyViolation.New())

	err := s.uc.Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, s.payload())
	s.Require().ErrorIs(err, mrauth.ErrEmailAlreadyExists)
}

// TestExecuteStorageError - прочие сбои хранилища остаются внутренними, а не выдаются
// за занятый адрес.
func (s *ChangeEmailSuite) TestExecuteStorageError() {
	s.storage.EXPECT().
		UpdateEmail(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.ErrInternalStorageQueryFailed.New())

	err := s.uc.Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, s.payload())
	s.Require().Error(err)
	s.Require().NotErrorIs(err, mrauth.ErrEmailAlreadyExists)
}
