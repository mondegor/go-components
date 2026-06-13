package bag

import (
	"net/http"
	"time"
)

type (
	// RefreshTokenCookie - чтение, установка и удаление HTTP cookie с refresh токеном (web-версия).
	RefreshTokenCookie struct {
		name   string
		domain string
		path   string
		expiry time.Duration
	}
)

// NewRefreshTokenCookie - создаёт объект RefreshTokenCookie.
func NewRefreshTokenCookie(name, domain, path string, expiry time.Duration) *RefreshTokenCookie {
	return &RefreshTokenCookie{
		name:   name,
		domain: domain,
		path:   path,
		expiry: expiry,
	}
}

// GetValue - возвращает значение refresh токена из cookie запроса или пустую строку, если её нет.
func (c *RefreshTokenCookie) GetValue(r *http.Request) (refreshToken string) {
	cookie, err := r.Cookie(c.name)
	if err != nil {
		return ""
	}

	return cookie.Value
}

// SetValue - устанавливает cookie с refresh токеном и сроком жизни expiry.
func (c *RefreshTokenCookie) SetValue(w http.ResponseWriter, refreshToken string) {
	ck := http.Cookie{
		Name:     c.name,
		Value:    refreshToken,
		Path:     c.path,
		Domain:   c.domain,
		Expires:  time.Now().Add(c.expiry),
		MaxAge:   int(c.expiry.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	http.SetCookie(w, &ck)
}

// RemoveValue - удаляет cookie с refresh токеном.
func (c *RefreshTokenCookie) RemoveValue(w http.ResponseWriter) {
	ck := http.Cookie{
		Name:     c.name,
		Value:    "",
		Path:     c.path,
		Domain:   c.domain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	http.SetCookie(w, &ck)
}
