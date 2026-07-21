package security_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

type ApplyOperationSuite struct {
	baseSuite

	storage *mock.MockoperationDeleter
	handler *mock.MockOperationHandler
	deleted string
}

func TestApplyOperationSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApplyOperationSuite))
}

func (s *ApplyOperationSuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.storage = mock.NewMockoperationDeleter(s.ctrl)
	s.handler = mock.NewMockOperationHandler(s.ctrl)
	s.deleted = ""

	s.storage.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token string) error {
			s.deleted = token

			return nil
		}).
		AnyTimes()
}

func (s *ApplyOperationSuite) newUseCase(handlers map[string]mrauth.OperationHandler) *security.ApplyOperation {
	return security.NewApplyOperation(s.txManager, s.storage, s.logOperation, handlers)
}

func (s *ApplyOperationSuite) TestNilUserID() {
	s.Require().Error(s.newUseCase(nil).Execute(s.ctx, dto.ActorMeta{}, "op-token"))
}

func (s *ApplyOperationSuite) TestSuccess() {
	userID := uuid.New()
	op := confirmedOp(userID, "{}")

	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)
	s.handler.EXPECT().Execute(gomock.Any(), userID, gomock.Any()).Return(nil)

	uc := s.newUseCase(map[string]mrauth.OperationHandler{"confirm.change.totp": s.handler})

	s.Require().NoError(uc.Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token"))
	s.Equal("op-token", s.deleted)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Applied, s.logEntries[0].LogStatus)
	s.Equal(logreason.Unspecified, s.logEntries[0].Reason)
	s.Equal(op.Name, s.logEntries[0].OperationName)
	s.Equal(userID, s.logEntries[0].VisitorID)
}

func (s *ApplyOperationSuite) TestWrongUser() {
	stranger := uuid.New()

	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(confirmedOp(uuid.New(), "{}"), nil)

	s.Require().Error(s.newUseCase(nil).Execute(s.ctx, dto.ActorMeta{VisitorID: stranger}, "op-token"))

	// в журнал попадает обратившийся, а не владелец операции
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.AccessForbidden, s.logEntries[0].Reason)
	s.Equal(stranger, s.logEntries[0].VisitorID)
}

func (s *ApplyOperationSuite) TestNotConfirmed() {
	userID := uuid.New()
	op := confirmedOp(userID, "{}")
	op.Status = operationstatus.Opened

	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)
	s.handler.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	uc := s.newUseCase(map[string]mrauth.OperationHandler{"confirm.change.totp": s.handler})

	s.Require().Error(uc.Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token"))
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.NotConfirmed, s.logEntries[0].Reason)
}

func (s *ApplyOperationSuite) TestUnknownName() {
	userID := uuid.New()

	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(confirmedOp(userID, "{}"), nil)

	uc := s.newUseCase(map[string]mrauth.OperationHandler{})

	s.Require().Error(uc.Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token"))

	// незарегистрированный обработчик - ошибка конфигурации, а не событие безопасности
	s.Empty(s.logEntries)
}
