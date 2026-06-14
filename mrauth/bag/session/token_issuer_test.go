package session_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/bag/session"
	"github.com/mondegor/go-components/mrauth/bag/session/mock"
	"github.com/mondegor/go-components/mrauth/dto"
)

//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth TokenGenerator

const (
	accessExpiry  = 15 * time.Minute
	refreshExpiry = 24 * time.Hour
)

func TestTokenIssuer_CreateTokenPair(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	gen := mock.NewMockTokenGenerator(ctrl)
	gen.EXPECT().GenToken().Return("access-token-value", nil)
	gen.EXPECT().GenToken().Return("refresh-token-value", nil)

	issuer := session.NewTokenIssuer(gen, accessExpiry, refreshExpiry)

	userScopes := dto.UserScopes{
		UserID:   uuid.New(),
		Realm:    "site/user",
		Kind:     "user",
		LangCode: "ru",
	}

	got, err := issuer.CreateTokenPair(userScopes)
	require.NoError(t, err)

	assert.False(t, got.Access.HasSignature)
	assert.Equal(t, "access-token-value", got.Access.Token)
	assert.Equal(t, accessExpiry, got.Access.ExpiresIn)
	assert.Equal(t, "refresh-token-value", got.Refresh.Token)
	assert.Equal(t, refreshExpiry, got.Refresh.ExpiresIn)
	assert.Equal(t, userScopes.UserID, got.UserID)
	assert.Equal(t, userScopes.Realm, got.Scopes.Realm)
	assert.Equal(t, userScopes.Kind, got.Scopes.UserKind)
	assert.Equal(t, userScopes.LangCode, got.Scopes.LangCode)
}

func TestTokenIssuer_CreateTokenPair_AccessError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	gen := mock.NewMockTokenGenerator(ctrl)
	gen.EXPECT().GenToken().Return("", errors.New("gen failed"))

	issuer := session.NewTokenIssuer(gen, accessExpiry, refreshExpiry)

	_, err := issuer.CreateTokenPair(dto.UserScopes{UserID: uuid.New()})
	require.Error(t, err)
}

func TestTokenIssuer_CreateTokenPair_RefreshError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	gen := mock.NewMockTokenGenerator(ctrl)
	gen.EXPECT().GenToken().Return("access-token-value", nil)
	gen.EXPECT().GenToken().Return("", errors.New("gen failed"))

	issuer := session.NewTokenIssuer(gen, accessExpiry, refreshExpiry)

	_, err := issuer.CreateTokenPair(dto.UserScopes{UserID: uuid.New()})
	require.Error(t, err)
}
