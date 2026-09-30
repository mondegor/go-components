package auth2fa_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/service/auth2fa"
	"github.com/mondegor/go-components/mrauth/service/auth2fa/mock"
	"github.com/mondegor/go-components/mrauth/service/notify"
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
	s.svc = auth2fa.NewRecoveryAlerter(
		s.notifierAPI,
		notify.NewActorProps(func(string) (string, string) {
			return "TestApp", "TestDevice"
		}),
		2,
	)
}

// TestNotifiesEveryUse - уведомление уходит на каждое использование кода с контекстом клиента,
// предъявившего код, а признак low выставляется, только когда остаток не выше порога.
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
			actor := dto.ActorMeta{UserID: userID, ClientIP: mrtype.NewIP(netip.MustParseAddr("192.0.2.10"))}

			s.notifierAPI.EXPECT().
				Send(gomock.Any(), "user.recovery_codes.used", gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, props map[string]any) error {
					s.Equal(userID, props["to"]) // получатель передаётся идентификатором пользователя
					s.Equal(tc.remaining, props["remaining"])
					s.Equal(tc.wantLow, props["low"])
					s.Equal("192.0.2.10", props["ip"])
					s.Equal("TestApp, TestDevice", props["device"])
					s.Contains(props, "occurredAt")

					return nil
				})

			s.Require().NoError(s.svc.SendAlert(s.ctx, actor, tc.remaining))
		})
	}
}
