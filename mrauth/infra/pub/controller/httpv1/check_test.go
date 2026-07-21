package httpv1_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/mock"
)

//go:generate mockgen -destination=mock/mrserver.go -package=mock github.com/mondegor/go-webcore/mrserver ResponseSender

type CheckSuite struct {
	suite.Suite

	ctrl   *gomock.Controller
	sender *mock.MockResponseSender
	status int
	body   []byte
}

func TestCheckSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(CheckSuite))
}

func (s *CheckSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *CheckSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.sender = mock.NewMockResponseSender(s.ctrl)
	s.status = 0
	s.body = nil

	// JWKS отдаётся только через SendBytes; вызов остальных методов - ошибка контроллера
	s.sender.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.sender.EXPECT().SendNoContent(gomock.Any()).Times(0)
}

// expectSendBytes - принимает ответ, отданный контроллером через SendBytes.
func (s *CheckSuite) expectSendBytes() {
	s.sender.EXPECT().
		SendBytes(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ http.ResponseWriter, status int, body []byte) error {
			s.status = status
			s.body = body

			return nil
		}).
		AnyTimes()
}

func (s *CheckSuite) getJwks(jwks []byte) error {
	controller := httpv1.NewCheck(nil, s.sender, nil, nil, jwks)

	return controller.GetJwks(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil),
	)
}

func (s *CheckSuite) TestGetJwks() {
	s.Run("success", func() {
		want := []byte(`{"keys":[{"kty":"RSA","kid":"k1","alg":"RS256"}]}`)

		s.expectSendBytes()

		s.Require().NoError(s.getJwks(want))
		s.Equal(http.StatusOK, s.status)
		s.JSONEq(string(want), string(s.body))
	})

	s.Run("session-only (nil body) - not found, no panic", func() {
		s.expectSendBytes()

		s.Require().Error(s.getJwks(nil))
		s.Nil(s.body)
	})
}

func (s *CheckSuite) TestHandlersJwksRegistered() {
	controller := httpv1.NewCheck(nil, nil, nil, nil, nil)

	var hasJwks bool

	for _, h := range controller.Handlers() {
		if h.URL == "/.well-known/jwks.json" {
			hasJwks = true
		}
	}

	s.True(hasJwks)
}
