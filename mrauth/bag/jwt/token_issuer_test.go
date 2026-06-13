package jwt_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/bag/jwt"
	"github.com/mondegor/go-components/mrauth/bag/jwt/mock"
	"github.com/mondegor/go-components/mrauth/dto"
)

const (
	accessExpiry  = 15 * time.Minute
	refreshExpiry = 24 * time.Hour
	secret        = "test-secret-value"
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

			issuer := jwt.NewTokenIssuer(gen, accessExpiry, refreshExpiry, tt.signingMethod, []byte(secret))

			userScopes := dto.UserScopes{
				UserID:   uuid.New(),
				Realm:    "site/admin",
				Kind:     "admin",
				LangCode: "en",
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

			// round-trip: access токен должен распаковываться тем же секретом
			parsed, err := jwt.NewParser(secret).Parse(got.Access.Token)
			require.NoError(t, err)
			assert.Equal(t, userScopes.UserID, parsed.UserID)
			assert.Equal(t, userScopes.Realm, parsed.Realm)
			assert.Equal(t, userScopes.Kind, parsed.Kind)
		})
	}
}

func TestTokenIssuer_CreateTokenPair_GeneratorError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	gen := mock.NewMockTokenGenerator(ctrl)
	gen.EXPECT().GenToken().Return("", errors.New("gen failed"))

	issuer := jwt.NewTokenIssuer(gen, accessExpiry, refreshExpiry, "HS512", []byte(secret))

	_, err := issuer.CreateTokenPair(dto.UserScopes{UserID: uuid.New()})
	require.Error(t, err)
}
