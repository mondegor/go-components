package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/bag/jwt"
	jwtmock "github.com/mondegor/go-components/mrauth/bag/jwt/mock"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/repository"
)

const jwtSecret = "test-secret-value"

func signedAccessToken(t *testing.T, scopes dto.UserScopes) string {
	t.Helper()

	ctrl := gomock.NewController(t)
	gen := jwtmock.NewMockTokenGenerator(ctrl)
	gen.EXPECT().GenToken().Return("refresh", nil)

	pair, err := jwt.NewTokenIssuer(gen, 15*time.Minute, 24*time.Hour, "HS512", []byte(jwtSecret)).
		CreateTokenPair(scopes)
	require.NoError(t, err)

	return pair.Access.Token
}

func TestAuthTokenJWT_FetchOneByAccessToken(t *testing.T) {
	t.Parallel()

	want := dto.UserScopes{
		UserID:   uuid.New(),
		Realm:    "site/admin",
		Kind:     "admin",
		LangCode: "en",
	}
	token := signedAccessToken(t, want)

	got, err := repository.NewAuthTokenJWT(jwtSecret).FetchOneByAccessToken(context.Background(), token)
	require.NoError(t, err)

	assert.Equal(t, want.UserID, got.UserID)
	assert.Equal(t, want.Realm, got.Realm)
	assert.Equal(t, want.Kind, got.Kind)
	assert.Equal(t, want.LangCode, got.LangCode)
}

func TestAuthTokenJWT_FetchOneByAccessToken_Invalid(t *testing.T) {
	t.Parallel()

	_, err := repository.NewAuthTokenJWT(jwtSecret).FetchOneByAccessToken(context.Background(), "not-a-jwt")
	require.Error(t, err)
}
