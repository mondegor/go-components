package bag_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
)

const cookieName = "RTID"

func newCookie() *bag.RefreshTokenCookie {
	return bag.NewRefreshTokenCookie(cookieName, "localhost", "/", 24*time.Hour)
}

func TestRefreshTokenCookie_GetValue(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		cookie *http.Cookie
		want   string
	}

	tests := []testCase{
		{name: "present", cookie: &http.Cookie{Name: cookieName, Value: "token-value"}, want: "token-value"},
		{name: "absent", cookie: nil, want: ""},
		{name: "other name", cookie: &http.Cookie{Name: "OTHER", Value: "x"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookie != nil {
				r.AddCookie(tt.cookie)
			}

			assert.Equal(t, tt.want, newCookie().GetValue(r))
		})
	}
}

func TestRefreshTokenCookie_SetValue(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	newCookie().SetValue(w, "new-token")

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	got := cookies[0]
	assert.Equal(t, cookieName, got.Name)
	assert.Equal(t, "new-token", got.Value)
	assert.Equal(t, "/", got.Path)
	assert.True(t, got.HttpOnly)
	assert.True(t, got.Secure)
	assert.Positive(t, got.MaxAge)
}

func TestRefreshTokenCookie_RemoveValue(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	newCookie().RemoveValue(w)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	got := cookies[0]
	assert.Equal(t, cookieName, got.Name)
	assert.Empty(t, got.Value)
	assert.Negative(t, got.MaxAge)
}
