package jwt_test

import (
	"errors"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/bag/jwt"
	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
	"github.com/mondegor/go-components/mrauth/bag/jwt/mock"
	"github.com/mondegor/go-components/mrauth/dto"
)

//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth TokenGenerator

// hmacKeySet - набор из одного HMAC-ключа без kid (применяется к токенам без 'kid').
func hmacKeySet(t *testing.T, signSecret string) crypt.KeySet {
	t.Helper()

	key, err := crypt.NewHMACKey("", "", []byte(signSecret))
	require.NoError(t, err)

	keySet, err := crypt.NewKeySet(key)
	require.NoError(t, err)

	return keySet
}

const (
	accessExpiry  = 15 * time.Minute
	refreshExpiry = 24 * time.Hour
	secret        = "test-secret-value"
	issuerName    = "https://auth.test"
)

func TestTokenIssuer_CreateTokenPair(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		signingMethod string
	}

	tests := []testCase{
		{name: "HS512", signingMethod: "HS512"},
		{name: "default HS256", signingMethod: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			gen := mock.NewMockTokenGenerator(ctrl)
			gen.EXPECT().GenToken().Return("refresh-token-value", nil)

			signingKey, err := crypt.NewHMACKey("", tt.signingMethod, []byte(secret))
			require.NoError(t, err)

			issuer := jwt.NewTokenIssuer(gen, accessExpiry, refreshExpiry, issuerName, signingKey)

			userScopes := dto.UserScopes{
				UserID:    uuid.New(),
				SessionID: 0x1f3bc817,
				Realm:     "site/admin",
				Kind:      "admin",
				LangCode:  "en",
			}

			got, err := issuer.CreateTokenPair(userScopes)
			require.NoError(t, err)

			assert.True(t, got.Access.HasSignature)
			assert.NotEmpty(t, got.Access.Token)
			assert.Equal(t, accessExpiry, got.Access.ExpiresIn)
			assert.Equal(t, "refresh-token-value", got.Refresh.Token)
			assert.Equal(t, refreshExpiry, got.Refresh.ExpiresIn)
			assert.Equal(t, userScopes.UserID, got.UserID)
			assert.Equal(t, userScopes.Realm, got.Scopes.Realm)
			assert.Equal(t, userScopes.Kind, got.Scopes.UserKind)
			assert.Equal(t, userScopes.LangCode, got.Scopes.LangCode)

			// round-trip: access токен должен распаковываться тем же ключом (тот же алгоритм)
			verifyKey, err := crypt.NewHMACKey("", tt.signingMethod, []byte(secret))
			require.NoError(t, err)

			verifyKeys, err := crypt.NewKeySet(verifyKey)
			require.NoError(t, err)

			parsed, err := jwt.NewParser(verifyKeys).Parse(got.Access.Token)
			require.NoError(t, err)
			assert.Equal(t, userScopes.UserID, parsed.UserID)
			assert.Equal(t, userScopes.SessionID, parsed.SessionID)
			assert.Equal(t, userScopes.Realm, parsed.Realm)
			assert.Equal(t, userScopes.Kind, parsed.Kind)

			// в выпущенном токене должны присутствовать claim'ы iss/iat/jti
			rawClaims := gojwt.MapClaims{}
			_, _, err = gojwt.NewParser().ParseUnverified(got.Access.Token, rawClaims)
			require.NoError(t, err)
			assert.Equal(t, issuerName, rawClaims["iss"])
			assert.Contains(t, rawClaims, "iat")
			assert.NotEmpty(t, rawClaims["jti"])
		})
	}
}

func TestTokenIssuer_CreateTokenPair_GeneratorError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	gen := mock.NewMockTokenGenerator(ctrl)
	gen.EXPECT().GenToken().Return("", errors.New("gen failed"))

	signingKey, err := crypt.NewHMACKey("", "HS512", []byte(secret))
	require.NoError(t, err)

	issuer := jwt.NewTokenIssuer(gen, accessExpiry, refreshExpiry, issuerName, signingKey)

	_, err = issuer.CreateTokenPair(dto.UserScopes{UserID: uuid.New()})
	require.Error(t, err)
}
