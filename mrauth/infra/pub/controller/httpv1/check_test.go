package httpv1_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
)

// bytesSenderStub - минимальный mrserver.ResponseSender для проверки SendBytes.
type bytesSenderStub struct {
	status int
	body   []byte
}

func (s *bytesSenderStub) Send(_ http.ResponseWriter, _ int, _ any) error {
	return errors.New("Send must not be called")
}

func (s *bytesSenderStub) SendBytes(_ http.ResponseWriter, status int, body []byte) error {
	s.status = status
	s.body = body

	return nil
}

func (s *bytesSenderStub) SendNoContent(_ http.ResponseWriter) error {
	return errors.New("SendNoContent must not be called")
}

func TestCheck_GetJwks(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		want := []byte(`{"keys":[{"kty":"RSA","kid":"k1","alg":"RS256"}]}`)
		sender := &bytesSenderStub{}
		controller := httpv1.NewCheck(nil, sender, nil, nil, want)

		err := controller.GetJwks(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, sender.status)
		assert.JSONEq(t, string(want), string(sender.body))
	})

	t.Run("session-only (nil body) - not found, no panic", func(t *testing.T) {
		t.Parallel()

		sender := &bytesSenderStub{}
		controller := httpv1.NewCheck(nil, sender, nil, nil, nil)

		err := controller.GetJwks(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))

		require.Error(t, err)
		assert.Nil(t, sender.body)
	})
}

func TestCheck_Handlers_JwksRegistered(t *testing.T) {
	t.Parallel()

	controller := httpv1.NewCheck(nil, nil, nil, nil, nil)

	var hasJwks bool

	for _, h := range controller.Handlers() {
		if h.URL == "/.well-known/jwks.json" {
			hasJwks = true
		}
	}

	assert.True(t, hasJwks)
}
