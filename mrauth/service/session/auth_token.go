package session

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// AuthToken - comment struct.
	AuthToken struct {
		storage           authTokenStorage
		errorWrapper      errors.Wrapper
		logger            mrlog.Logger
		realm2tokenIssuer map[string]mrauth.TokenIssuer
	}

	authTokenStorage interface {
		Insert(ctx context.Context, row entity.AuthToken) error
		Revoke(ctx context.Context, refreshToken string) (row dto.UserScopes, err error)
		UpdateToClose(ctx context.Context, accessToken string) error
	}

	// AuthTokenRealm - сообщение для получателя.
	AuthTokenRealm struct {
		Name        string
		TokenIssuer mrauth.TokenIssuer
	}
)

// NewAuthToken - создаёт объект AuthToken.
func NewAuthToken(
	storage authTokenStorage,
	logger mrlog.Logger,
	allowedRealms []AuthTokenRealm,
) *AuthToken {
	realm2tokenIssuer := make(map[string]mrauth.TokenIssuer, len(allowedRealms))
	for _, realm := range allowedRealms {
		realm2tokenIssuer[realm.Name] = realm.TokenIssuer
	}

	return &AuthToken{
		storage:           storage,
		errorWrapper:      errors.NewServiceOperationFailedWrapper(),
		logger:            logger,
		realm2tokenIssuer: realm2tokenIssuer,
	}
}

// Create - comments method.
func (sv *AuthToken) Create(ctx context.Context, userScopes dto.UserScopes) (token dto.AuthToken, err error) {
	tokenIssuer, ok := sv.realm2tokenIssuer[userScopes.Realm]
	if !ok {
		return dto.AuthToken{}, errors.ErrIncorrectInputData.New("realm is unknown")
	}

	token, err = tokenIssuer.Create(userScopes)
	if err != nil {
		return dto.AuthToken{}, sv.errorWrapper.Wrap(err)
	}

	authToken := entity.AuthToken{
		AccessToken:     token.AccessToken,
		RefreshToken:    token.RefreshToken,
		AccessExpiresAt: time.Now().Add(token.ExpiresIn).Round(1 * time.Second),
		UserID:          token.UserID,
		Scopes:          token.Scopes,
		ExpiresAt:       time.Now().Add(token.RefreshExpiresIn).Round(1 * time.Second),
	}

	// accessToken сохраняется в БД только у сессионных токенов,
	// подписанные токены типа jwt распаковываются без обращения к БД
	if token.HasSignature {
		authToken.AccessToken = ""
	}

	if err = sv.storage.Insert(ctx, authToken); err != nil {
		return dto.AuthToken{}, sv.errorWrapper.Wrap(err)
	}

	return token, nil
}

// Revoke - comments method.
func (sv *AuthToken) Revoke(ctx context.Context, refreshToken string) (row dto.UserScopes, err error) {
	row, err = sv.storage.Revoke(ctx, refreshToken)
	if err != nil {
		return dto.UserScopes{}, sv.errorWrapper.Wrap(err)
	}

	return row, nil
}

// Close - comments method.
func (sv *AuthToken) Close(ctx context.Context, accessToken string) error {
	if err := sv.storage.UpdateToClose(ctx, accessToken); err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	return nil
}
