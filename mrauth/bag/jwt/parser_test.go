package jwt_test

import (
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/jwt"
)

// validClaims - возвращает набор claim'ов корректного access токена.
func validClaims() gojwt.MapClaims {
	return gojwt.MapClaims{
		"aud":   "site/admin",
		"sub":   uuid.New().String(),
		"sid":   "523266583",
		"lan":   "en",
		"scope": "admin",
		"exp":   gojwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
	}
}

// signToken - подписывает claim'ы секретом secret методом HS256.
func signToken(t *testing.T, claims gojwt.MapClaims, signSecret string) string {
	t.Helper()

	token, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString([]byte(signSecret))
	require.NoError(t, err)

	return token
}

func TestParser_Parse_Valid(t *testing.T) {
	t.Parallel()

	claims := validClaims()
	token := signToken(t, claims, secret)

	got, err := jwt.NewParser(hmacKeySet(t, secret)).Parse(token)
	require.NoError(t, err)

	assert.Equal(t, claims["sub"], got.UserID.String())
	assert.Equal(t, uint32(523266583), got.SessionID)
	assert.Equal(t, "site/admin", got.Realm)
	assert.Equal(t, "admin", got.Kind)
	assert.Equal(t, "en", got.LangCode)
}

func TestParser_Parse_SectionInvalid(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		mutate func(claims gojwt.MapClaims)
	}

	tests := []testCase{
		{name: "aud absent", mutate: func(c gojwt.MapClaims) { delete(c, "aud") }},
		{name: "sub absent", mutate: func(c gojwt.MapClaims) { delete(c, "sub") }},
		{name: "sid absent", mutate: func(c gojwt.MapClaims) { delete(c, "sid") }},
		{name: "lan absent", mutate: func(c gojwt.MapClaims) { delete(c, "lan") }},
		{name: "scope absent", mutate: func(c gojwt.MapClaims) { delete(c, "scope") }},
		{name: "aud empty", mutate: func(c gojwt.MapClaims) { c["aud"] = "" }},
		{name: "sid empty", mutate: func(c gojwt.MapClaims) { c["sid"] = "" }},
		{name: "lan empty", mutate: func(c gojwt.MapClaims) { c["lan"] = "" }},
		{name: "scope empty", mutate: func(c gojwt.MapClaims) { c["scope"] = "" }},
		{name: "sub not uuid", mutate: func(c gojwt.MapClaims) { c["sub"] = "not-a-uuid" }},
		{name: "sid not numeric", mutate: func(c gojwt.MapClaims) { c["sid"] = "abc" }},
		{name: "scope wrong type", mutate: func(c gojwt.MapClaims) { c["scope"] = 123 }},
		// парсер трактует 'aud' как одиночную строку (realm); массив (формат RFC 7519) не поддерживается
		{name: "aud as array", mutate: func(c gojwt.MapClaims) { c["aud"] = []string{"site/admin"} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			claims := validClaims()
			tt.mutate(claims)
			token := signToken(t, claims, secret)

			_, err := jwt.NewParser(hmacKeySet(t, secret)).Parse(token)
			require.ErrorIs(t, err, jwt.ErrTokenSectionInvalid)
		})
	}
}

func TestParser_Parse_Expired(t *testing.T) {
	t.Parallel()

	claims := validClaims()
	claims["exp"] = gojwt.NewNumericDate(time.Now().Add(-time.Minute))
	token := signToken(t, claims, secret)

	_, err := jwt.NewParser(hmacKeySet(t, secret)).Parse(token)
	require.ErrorIs(t, err, jwt.ErrTokenExpired)
}

func TestParser_Parse_WithinLeeway(t *testing.T) {
	t.Parallel()

	// токен истёк недавно, но в пределах допустимого расхождения часов (parseLeeway) - принимается
	claims := validClaims()
	claims["exp"] = gojwt.NewNumericDate(time.Now().Add(-20 * time.Second))
	token := signToken(t, claims, secret)

	_, err := jwt.NewParser(hmacKeySet(t, secret)).Parse(token)
	require.NoError(t, err)
}

func TestParser_Parse_WrongSecret(t *testing.T) {
	t.Parallel()

	token := signToken(t, validClaims(), "another-secret-value")

	_, err := jwt.NewParser(hmacKeySet(t, secret)).Parse(token)
	require.ErrorIs(t, err, jwt.ErrTokenInvalid)
}
