package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/jwt"
	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
	jwtmock "github.com/mondegor/go-components/mrauth/bag/jwt/mock"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/repository"
)

const jwtSecret = "test-secret-value"

func signedAccessToken(t *testing.T, scopes dto.UserScopes) string {
	t.Helper()

	return signedAccessTokenWithTTL(t, scopes, 15*time.Minute)
}

func signedAccessTokenWithTTL(t *testing.T, scopes dto.UserScopes, accessTTL time.Duration) string {
	t.Helper()

	ctrl := gomock.NewController(t)
	gen := jwtmock.NewMockTokenGenerator(ctrl)
	gen.EXPECT().GenToken().Return("refresh", nil)

	signingKey, err := crypt.NewHMACKey("", "HS512", []byte(jwtSecret))
	require.NoError(t, err)

	pair, err := jwt.NewTokenIssuer(gen, accessTTL, 24*time.Hour, "https://auth.test", signingKey).
		CreateTokenPair(scopes)
	require.NoError(t, err)

	return pair.Access.Token
}

// validScopes - минимально полная область действия: издатель токена требует все секции.
func validScopes() dto.UserScopes {
	return dto.UserScopes{
		UserID:    uuid.New(),
		SessionID: 523266583,
		Realm:     "site/admin",
		Kind:      "admin",
		LangCode:  "en",
		TimeZone:  "Europe/Moscow",
	}
}

// verifyKeySet - набор ключей для проверки подписи токенов, выпущенных signedAccessToken.
func verifyKeySet(t *testing.T) crypt.KeySet {
	t.Helper()

	verifyKey, err := crypt.NewHMACKey("", "HS512", []byte(jwtSecret))
	require.NoError(t, err)

	keys, err := crypt.NewKeySet(verifyKey)
	require.NoError(t, err)

	return keys
}

func TestAuthTokenJWT_FetchOneByAccessToken(t *testing.T) {
	t.Parallel()

	want := validScopes()
	token := signedAccessToken(t, want)

	got, err := repository.NewAuthTokenJWT(verifyKeySet(t)).FetchOneByAccessToken(context.Background(), token)
	require.NoError(t, err)

	assert.Equal(t, want.UserID, got.UserID)
	assert.Equal(t, want.Realm, got.Realm)
	assert.Equal(t, want.Kind, got.Kind)
	assert.Equal(t, want.LangCode, got.LangCode)
	assert.Equal(t, want.TimeZone, got.TimeZone)
}

// TestAuthTokenJWT_FetchOneByAccessToken_Invalid - неразобравшийся токен приводится к сентинелу этого пакета.
func TestAuthTokenJWT_FetchOneByAccessToken_Invalid(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name  string
		token string
	}

	tests := []testCase{
		{name: "не jwt вовсе", token: "not-a-jwt"},
		{name: "чужая подпись", token: signedForeignAccessToken(t)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := repository.NewAuthTokenJWT(verifyKeySet(t)).FetchOneByAccessToken(context.Background(), tt.token)
			require.ErrorIs(t, err, mrauth.ErrTokenInvalid)
			require.NotErrorIs(t, err, mrauth.ErrEventTokenExpired)
		})
	}
}

// TestAuthTokenJWT_FetchOneByAccessToken_Expired - истёкший токен приводится к тому же сентинелу,
// которым отвечает AuthTokenPostgres: обе реализации mrauth.AuthTokenFetcher подставляются
// в один UserProvider, поэтому истёкший access токен обязан давать один и тот же ответ клиенту
// независимо от того, какая из них подключена.
func TestAuthTokenJWT_FetchOneByAccessToken_Expired(t *testing.T) {
	t.Parallel()

	// TTL с запасом больше parseLeeway, иначе токен был бы принят как «часы разошлись»
	token := signedAccessTokenWithTTL(t, validScopes(), -10*time.Minute)

	_, err := repository.NewAuthTokenJWT(verifyKeySet(t)).FetchOneByAccessToken(context.Background(), token)
	require.ErrorIs(t, err, mrauth.ErrEventTokenExpired)
	require.NotErrorIs(t, err, mrauth.ErrTokenInvalid)
}

// signedForeignAccessToken - выпускает корректный по структуре токен, подписанный другим секретом.
func signedForeignAccessToken(t *testing.T) string {
	t.Helper()

	ctrl := gomock.NewController(t)
	gen := jwtmock.NewMockTokenGenerator(ctrl)
	gen.EXPECT().GenToken().Return("refresh", nil)

	signingKey, err := crypt.NewHMACKey("", "HS512", []byte("another-secret-value"))
	require.NoError(t, err)

	pair, err := jwt.NewTokenIssuer(gen, 15*time.Minute, 24*time.Hour, "https://auth.test", signingKey).
		CreateTokenPair(validScopes())
	require.NoError(t, err)

	return pair.Access.Token
}
