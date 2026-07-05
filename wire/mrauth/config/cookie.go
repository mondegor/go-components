package config

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	defaultCookieName   = "RTID"
	defaultCookiePath   = "/"
	defaultCookieExpiry = 180 * 24 * time.Hour
)

type (
	// RefreshCookieResolved - настройки refresh-cookie с подставленными безопасными дефолтами
	// и провалидированной комбинацией Secure/SameSite, готовые к передаче в конструктор cookie.
	RefreshCookieResolved struct {
		Name     string
		Domain   string
		Path     string
		Expiry   time.Duration
		Secure   bool
		SameSite http.SameSite
	}
)

// ResolveRefreshCookie - подставляет безопасные дефолты в незаданные поля cfg, парсит SameSite
// и проверяет инварианты безопасности cookie:
//   - Domain обязателен (host-приложение задаёт его явно);
//   - безопасные дефолты: Secure=true (если флаг не задан) и SameSite=Strict (если пусто);
//   - SameSite=None требует Secure=true - браузеры игнорируют None-cookie без Secure.
//
// Внимание: SameSite=Lax/None расширяют CSRF-поверхность refresh-эндпоинта и требуют отдельной
// CSRF-защиты на стороне host-приложения.
func ResolveRefreshCookie(cfg RefreshCookie) (RefreshCookieResolved, error) {
	if cfg.Domain == "" {
		return RefreshCookieResolved{}, errors.New("refresh token cookie domain is required")
	}

	sameSite, err := parseCookieSameSite(cfg.SameSite)
	if err != nil {
		return RefreshCookieResolved{}, err
	}

	// безопасный дефолт: Secure=true, если флаг не задан явно
	secure := cfg.Secure == nil || *cfg.Secure

	if sameSite == http.SameSiteNoneMode && !secure {
		return RefreshCookieResolved{}, errors.New("refresh token cookie: same_site=none requires secure=true")
	}

	out := RefreshCookieResolved{
		Name:     cfg.Name,
		Domain:   cfg.Domain,
		Path:     cfg.Path,
		Expiry:   cfg.Expiry,
		Secure:   secure,
		SameSite: sameSite,
	}

	if out.Name == "" {
		out.Name = defaultCookieName
	}

	if out.Path == "" {
		out.Path = defaultCookiePath
	}

	if out.Expiry < 1 {
		out.Expiry = defaultCookieExpiry
	}

	return out, nil
}

// parseCookieSameSite - преобразует строковое значение SameSite в http.SameSite.
// Пустая строка даёт безопасный дефолт (Strict); неизвестное значение - ошибку.
func parseCookieSameSite(value string) (http.SameSite, error) {
	switch strings.ToLower(value) {
	case "":
		return http.SameSiteStrictMode, nil
	case "strict":
		return http.SameSiteStrictMode, nil
	case "lax":
		return http.SameSiteLaxMode, nil
	case "none":
		return http.SameSiteNoneMode, nil
	default:
		return 0, fmt.Errorf("invalid refresh token cookie same_site: %q", value)
	}
}
