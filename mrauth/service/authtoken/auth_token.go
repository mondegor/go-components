package authtoken

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/authtokentype"
)

const (
	// revokeGracePeriod - окно действия отозванного refresh токена по умолчанию.
	revokeGracePeriod = 60 * time.Second
)

type (
	// AuthToken - сервис выпуска, ротации и отзыва пар токенов авторизации сессии.
	AuthToken struct {
		txManager    mrstorage.DBTxManager
		storage      authTokenStorage
		errorWrapper errors.Wrapper
		logger       mrlog.Logger
		realm2props  map[string]Realm
		gracePeriod  time.Duration
	}

	authTokenStorage interface {
		Insert(ctx context.Context, rows []entity.AuthToken) error
		RevokeRefresh(ctx context.Context, refreshToken string, grace time.Duration) (row dto.UserScopes, isRetried bool, err error)
		FetchLastEnabledPairBySessionID(ctx context.Context, userID uuid.UUID, sessionID uint32) (access, refresh entity.AuthToken, err error)
		RevokeSessionByRefreshToken(ctx context.Context, refreshToken string) error
	}

	// Realm - сопоставление идентификатора realm с его издателем токенов (TokenIssuer).
	Realm struct {
		ID          uint16
		TokenIssuer mrauth.TokenIssuer
	}
)

// New - создаёт объект AuthToken.
func New(
	txManager mrstorage.DBTxManager,
	storage authTokenStorage,
	realmRegistry mrauth.RealmRegistry,
	logger mrlog.Logger,
	allowedRealms []Realm,
) *AuthToken {
	realm2props := make(map[string]Realm, len(allowedRealms))
	for _, realm := range allowedRealms {
		realmName, ok := realmRegistry.NameByID(realm.ID)
		if !ok {
			continue
		}

		realm2props[realmName] = realm
	}

	return &AuthToken{
		txManager:    txManager,
		storage:      storage,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
		logger:       logger,
		realm2props:  realm2props,
		gracePeriod:  revokeGracePeriod,
	}
}

// Create - выпускает новую пару токенов.
func (sv *AuthToken) Create(ctx context.Context, userScopes dto.UserScopes) (tokenPair dto.AuthTokenPair, err error) {
	if userScopes.SessionID == 0 {
		return dto.AuthTokenPair{}, errors.ErrIncorrectInputData.New("userScopes.SessionID is required")
	}

	realmProps, ok := sv.realm2props[userScopes.Realm]
	if !ok {
		return dto.AuthTokenPair{}, errors.ErrIncorrectInputData.New("realm is unknown")
	}

	tokenPair, err = realmProps.TokenIssuer.CreateTokenPair(userScopes)
	if err != nil {
		return dto.AuthTokenPair{}, sv.errorWrapper.Wrap(err)
	}

	items := make([]entity.AuthToken, 0, 2)
	now := time.Now().UTC() // общий момент отсчёта: сроки обоих токенов пары должны быть согласованы

	items = append(
		items,
		entity.AuthToken{
			Token:     tokenPair.Refresh.Token,
			Type:      authtokentype.Refresh,
			UserID:    tokenPair.UserID,
			RealmID:   realmProps.ID,
			SessionID: userScopes.SessionID,
			Scopes:    tokenPair.Scopes,
			ExpiresAt: now.Add(tokenPair.Refresh.ExpiresIn).Round(1 * time.Second),
		},
	)

	// access токен сохраняется в БД только у сессионных токенов,
	// подписанные токены типа jwt распаковываются без обращения к БД
	if !tokenPair.Access.HasSignature {
		items = append(
			items,
			entity.AuthToken{
				Token:     tokenPair.Access.Token,
				Type:      authtokentype.Access,
				UserID:    tokenPair.UserID,
				RealmID:   realmProps.ID,
				SessionID: userScopes.SessionID,
				Scopes:    tokenPair.Scopes,
				ExpiresAt: now.Add(tokenPair.Access.ExpiresIn).Round(1 * time.Second),
			},
		)
	}

	if err = sv.storage.Insert(ctx, items); err != nil {
		return dto.AuthTokenPair{}, sv.errorWrapper.Wrap(err)
	}

	return tokenPair, nil
}

// Recreate - отзывает действующий токен и выпускает новую пару в той же сессии,
// либо (при повторе в окне действия) возвращает последнюю пару сессии.
// Отзыв и выпуск новой пары выполняются атомарно в одной транзакции.
func (sv *AuthToken) Recreate(ctx context.Context, refreshToken string) (token dto.AuthTokenPair, err error) {
	err = sv.txManager.Do(ctx, func(ctx context.Context) error {
		scopes, isRetried, err := sv.storage.RevokeRefresh(ctx, refreshToken, sv.gracePeriod)
		if err != nil {
			// sentinel-ошибки (TokenAlreadyRevokedError, ErrTokenExpired, ErrEventStorageNoRecordFound)
			// возвращаются без обёртки, чтобы вызывающий код мог их распознать через errors.As/Is
			return err
		}

		// при повторном обращении возвращается последняя пара токенов сессии
		if isRetried {
			token, err = sv.lastSessionToken(ctx, scopes)

			return err
		}

		// выпуск новой пары подписывает токен асимметричным ключом (RSA/JWT) - операция
		// CPU-bound, и здесь она выполняется ВНУТРИ транзакции, удерживая соединение из пула
		// и row-lock отозванного refresh токена, что удлиняет транзакцию и снижает пропускную
		// способность пула под нагрузкой. Осознанный trade-off: удержание принято ради
		// атомарности отзыва и выпуска новой пары. Вынос подписи из транзакции отложен до
		// появления давления на пул (ср. session_open.go, путь логина).
		token, err = sv.Create(ctx, scopes)

		return err
	})
	if err != nil {
		return dto.AuthTokenPair{}, err
	}

	return token, nil
}

func (sv *AuthToken) lastSessionToken(ctx context.Context, userScopes dto.UserScopes) (dto.AuthTokenPair, error) {
	access, refresh, err := sv.storage.FetchLastEnabledPairBySessionID(ctx, userScopes.UserID, userScopes.SessionID)
	if err != nil {
		return dto.AuthTokenPair{}, sv.errorWrapper.Wrap(err)
	}

	token := dto.AuthTokenPair{
		Refresh: dto.RefreshToken{
			Token:     refresh.Token,
			ExpiresIn: time.Until(refresh.ExpiresAt).Round(1 * time.Second),
		},
		UserID: userScopes.UserID,
		Scopes: refresh.Scopes,
	}

	// если access пустой, то значит это JWT токен, поэтому он перевыпускается
	if access.Token == "" {
		realmProps, ok := sv.realm2props[userScopes.Realm]
		if !ok {
			return dto.AuthTokenPair{}, errors.ErrIncorrectInputData.New("realm is unknown")
		}

		pair, err := realmProps.TokenIssuer.CreateTokenPair(userScopes)
		if err != nil {
			return dto.AuthTokenPair{}, sv.errorWrapper.Wrap(err)
		}

		if !pair.Access.HasSignature {
			return dto.AuthTokenPair{}, errors.ErrInternalIncorrectInputData.New("pair.Access.HasSignature = false")
		}

		token.Access = dto.AccessToken{
			Token:        pair.Access.Token,
			ExpiresIn:    pair.Access.ExpiresIn,
			HasSignature: true,
		}

		return token, nil
	}

	token.Access = dto.AccessToken{
		Token:     access.Token,
		ExpiresIn: time.Until(access.ExpiresAt).Round(1 * time.Second),
	}

	return token, nil
}

// Close - отзывает все действующие токены сессии по её refresh токену (logout).
func (sv *AuthToken) Close(ctx context.Context, refreshToken string) error {
	if err := sv.storage.RevokeSessionByRefreshToken(ctx, refreshToken); err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	return nil
}
