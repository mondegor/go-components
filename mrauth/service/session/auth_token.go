package session

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// AuthToken - comment struct.
	AuthToken struct {
		storage           mrauth.AuthTokenStorage
		errorWrapper      errors.Wrapper
		logger            mrlog.Logger
		realm2tokenIssuer map[string]mrauth.TokenIssuer
	}

	// AuthTokenRealm - сообщение для получателя.
	AuthTokenRealm struct {
		Name        string
		TokenIssuer mrauth.TokenIssuer
	}
)

// NewAuthToken - создаёт объект AuthToken.
func NewAuthToken(
	storage mrauth.AuthTokenStorage,
	logger mrlog.Logger,
	allowedRealms []AuthTokenRealm,
) *AuthToken {
	realm2tokenIssuer := make(map[string]mrauth.TokenIssuer, len(allowedRealms))
	for _, realm := range allowedRealms {
		realm2tokenIssuer[realm.Name] = realm.TokenIssuer
	}

	return &AuthToken{
		storage:           storage,
		errorWrapper:      errors.NewServiceWrapper(),
		logger:            logger,
		realm2tokenIssuer: realm2tokenIssuer,
	}
}

// Create - comments method.
func (uc *AuthToken) Create(ctx context.Context, realm, userKind, langCode string, userID uuid.UUID) (token dto.AuthToken, err error) {
	tokenIssuer, ok := uc.realm2tokenIssuer[realm]
	if !ok {
		return dto.AuthToken{}, errors.ErrUseCaseIncorrectInputData.New("realm is unknown")
	}

	token, err = tokenIssuer.Create(realm, userKind, langCode, userID)
	if err != nil {
		return dto.AuthToken{}, uc.errorWrapper.Wrap(err)
	}

	authToken := entity.AuthToken{
		AccessToken:     token.AccessToken,
		RefreshToken:    token.RefreshToken,
		AccessExpiresAt: time.Now().Add(token.ExpiresIn).Round(1 * time.Second),
		Scopes: dto.AuthTokenScopes{
			Realm:    token.Scopes.Realm,
			UserKind: token.Scopes.UserKind,
			LangCode: token.Scopes.LangCode,
			UserID:   token.Scopes.UserID,
		},
		ExpiresAt: time.Now().Add(token.RefreshExpiresIn).Round(1 * time.Second),
	}

	// accessToken сохраняется в БД только у сессионных токенов,
	// подписанные токены типа jwt распаковываются без обращения к БД
	if token.HasSignature {
		authToken.AccessToken = ""
	}

	if err = uc.storage.Insert(ctx, authToken); err != nil {
		return dto.AuthToken{}, uc.errorWrapper.Wrap(err)
	}

	return token, nil
}

// Revoke - comments method.
func (uc *AuthToken) Revoke(ctx context.Context, refreshToken string) (row dto.AuthTokenScopes, err error) {
	row, err = uc.storage.Revoke(ctx, refreshToken)
	if err != nil {
		return dto.AuthTokenScopes{}, uc.errorWrapper.Wrap(err)
	}

	return row, nil
}

// Close - comments method.
func (uc *AuthToken) Close(ctx context.Context, accessToken string) error {
	if err := uc.storage.UpdateToClose(ctx, accessToken); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
