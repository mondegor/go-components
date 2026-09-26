package auth2fa_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/service/auth2fa"
	"github.com/mondegor/go-components/mrauth/service/auth2fa/mock"
)

//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth Notifier

type RecoveryAlerterSuite struct {
	suite.Suite

	ctrl        *gomock.Controller
	ctx         context.Context
	notifierAPI *mock.MockNotifier
	svc         *auth2fa.RecoveryAlerter
}

func TestRecoveryAlerterSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(RecoveryAlerterSuite))
}

func (s *RecoveryAlerterSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.notifierAPI = mock.NewMockNotifier(s.ctrl)
	s.svc = auth2fa.NewRecoveryAlerter(s.notifierAPI, 2)
}

func (s *RecoveryAlerterSuite) TestAtThresholdSends() {
	userID := uuid.New()

	s.notifierAPI.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, props map[string]any) error {
			s.Equal(userID, props["to"]) // ID пользователя заменяется на email декоратором notifierAPI
			s.Equal(2, props["remaining"])

			return nil
		})

	s.Require().NoError(s.svc.SendAlert(s.ctx, userID, 2)) // остаток == порога
}

func (s *RecoveryAlerterSuite) TestAboveThresholdSkips() {
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	s.Require().NoError(s.svc.SendAlert(s.ctx, uuid.New(), 3)) // остаток > порога
}
