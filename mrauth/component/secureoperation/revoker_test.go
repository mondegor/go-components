package secureoperation_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/component/secureoperation/mock"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
)

//go:generate mockgen -source=revoker.go -destination=mock/revoker.go -package=mock

type RevokerSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	ctx          context.Context
	storage      *mock.MockoperationRevokerStorage
	logOperation *mock.MockoperationLogger
	logEntries   []entity.SecureOperationLog
	svc          *secureoperation.Revoker
}

func TestRevokerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(RevokerSuite))
}

func (s *RevokerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.storage = mock.NewMockoperationRevokerStorage(s.ctrl)
	s.logOperation = mock.NewMockoperationLogger(s.ctrl)
	s.logEntries = nil

	s.logOperation.EXPECT().
		Log(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.SecureOperationLog) {
			s.logEntries = append(s.logEntries, entry)
		}).
		AnyTimes()

	s.svc = secureoperation.NewRevoker(s.storage, s.logOperation)
}

// TestRevokeAll - все операции пользователя удаляются, а в журнал отзыв пишется по одному разу
// на каждый тип: две операции одного типа - одно событие, а не две одинаковые записи.
func (s *RevokerSuite) TestRevokeAll() {
	userID := uuid.New()

	s.storage.EXPECT().
		DeleteByUserID(gomock.Any(), userID).
		Return([]string{"confirm.authorize.user", "confirm.change.phone", "confirm.authorize.user"}, nil)

	err := s.svc.RevokeAll(s.ctx, dto.ActorMeta{VisitorID: userID}, logreason.Auth2FAStateChanged)
	s.Require().NoError(err)

	s.Require().Len(s.logEntries, 2)

	names := make([]string, 0, len(s.logEntries))

	for _, entry := range s.logEntries {
		names = append(names, entry.OperationName)
		s.Equal(userID, entry.VisitorID)
		s.Equal(logstatus.Revoked, entry.LogStatus)
		s.Equal(logreason.Auth2FAStateChanged, entry.Reason)
	}

	s.ElementsMatch([]string{"confirm.authorize.user", "confirm.change.phone"}, names)
}

// TestNothingToRevoke - у пользователя нет операций: не ошибка и без записей в журнале.
func (s *RevokerSuite) TestNothingToRevoke() {
	s.storage.EXPECT().DeleteByUserID(gomock.Any(), gomock.Any()).Return([]string{}, nil)

	s.Require().NoError(s.svc.RevokeAll(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, logreason.Auth2FAStateChanged))
	s.Empty(s.logEntries)
}

// TestStorageError - ошибка хранилища поднимается (отменяет изменение, в транзакции которого
// идёт отзыв), журнал не пишется.
func (s *RevokerSuite) TestStorageError() {
	s.storage.EXPECT().DeleteByUserID(gomock.Any(), gomock.Any()).Return(nil, errors.New("db is down"))

	s.Require().Error(s.svc.RevokeAll(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, logreason.Auth2FAStateChanged))
	s.Empty(s.logEntries)
}

// TestEmptyUserID - отзывать операции неизвестно чьи нельзя: пустой пользователь - нарушение
// инварианта (мок DeleteByUserID без EXPECT: любой вызов провалит тест).
func (s *RevokerSuite) TestEmptyUserID() {
	err := s.svc.RevokeAll(s.ctx, dto.ActorMeta{}, logreason.Auth2FAStateChanged)
	s.Require().ErrorIs(err, errors.ErrInternalIncorrectInputData)
}
