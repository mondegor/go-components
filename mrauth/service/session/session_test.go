package session_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service/session"
	"github.com/mondegor/go-components/mrauth/service/session/mock"
)

//go:generate mockgen -source=issuer.go -destination=mock/issuer.go -package=mock

// maxInsertAttempts дублирует одноимённую константу пакета session (внешний тестовый пакет
// не имеет к ней доступа) и фиксирует число попыток подбора свободного session_id.
const maxInsertAttempts = 3

type IssuerSuite struct {
	suite.Suite

	ctrl    *gomock.Controller
	ctx     context.Context
	storage *mock.MocksessionStorage
	svc     *session.Issuer
}

func TestIssuerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(IssuerSuite))
}

func (s *IssuerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.storage = mock.NewMocksessionStorage(s.ctrl)
	s.svc = session.NewIssuer(s.storage)
}

func (s *IssuerSuite) TestHappy() {
	s.storage.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)

	id, err := s.svc.Issue(s.ctx, entity.Session{})
	s.Require().NoError(err)
	s.NotZero(id) // session_id из диапазона [1, math.MaxUint32]
}

func (s *IssuerSuite) TestCollisionThenSuccess() {
	// первый подобранный session_id занят -> сервис перегенерирует id и повторяет вставку
	gomock.InOrder(
		s.storage.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(repository.ErrSessionIDCollision),
		s.storage.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil),
	)

	id, err := s.svc.Issue(s.ctx, entity.Session{})
	s.Require().NoError(err)
	s.NotZero(id)
}

func (s *IssuerSuite) TestCollisionExhausted() {
	// session_id занят на всех попытках -> возвращается ErrSessionIDCollision
	s.storage.EXPECT().Insert(gomock.Any(), gomock.Any()).
		Return(repository.ErrSessionIDCollision).Times(maxInsertAttempts)

	_, err := s.svc.Issue(s.ctx, entity.Session{})
	s.Require().ErrorIs(err, repository.ErrSessionIDCollision)
}

func (s *IssuerSuite) TestInsertOtherErrorNoRetry() {
	// прочая ошибка вставки не ретраится и возвращается как есть
	insertErr := errors.New("db down")
	s.storage.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(insertErr)

	_, err := s.svc.Issue(s.ctx, entity.Session{})
	s.Require().ErrorIs(err, insertErr)
}
