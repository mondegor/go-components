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

// TestNotifiesEveryUse - уведомление уходит на каждое использование кода, а признак low
// выставляется, только когда остаток не выше порога.
func (s *RecoveryAlerterSuite) TestNotifiesEveryUse() {
	tests := []struct {
		name      string
		remaining int
		wantLow   bool
	}{
		{name: "above threshold", remaining: 3, wantLow: false},
		{name: "at threshold", remaining: 2, wantLow: true},
		{name: "none left", remaining: 0, wantLow: true},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			userID := uuid.New()

			s.notifierAPI.EXPECT().
				Send(gomock.Any(), "user.recovery_codes.used", gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, props map[string]any) error {
					s.Equal(userID, props["to"]) // ID пользователя заменяется на email декоратором notifierAPI
					s.Equal(tc.remaining, props["remaining"])
					s.Equal(tc.wantLow, props["low"])

					return nil
				})

			s.Require().NoError(s.svc.SendAlert(s.ctx, userID, tc.remaining))
		})
	}
}
