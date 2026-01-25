package mrauth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// OperationUseCase - comments interface.
	OperationUseCase interface {
		ConfirmAction(ctx context.Context, operationToken, secret string) (entity.SecureOperation, error)
		ResendCode(ctx context.Context, operationToken string) (entity.SecureOperation, error)
		Revoke(ctx context.Context, operationToken string) error
	}

	// AuthTokenFetcher - предоставляет доступ к хранилищу сообщений.
	AuthTokenFetcher interface {
		FetchOne(ctx context.Context, accessToken string) (dto.AuthTokenScopes, error)
	}

	// AuthTokenStorage - comments interface.
	AuthTokenStorage interface {
		Insert(ctx context.Context, row entity.AuthToken) error
		Revoke(ctx context.Context, refreshToken string) (row dto.AuthTokenScopes, err error)
		UpdateToClose(ctx context.Context, accessToken string) error
		UpdateToCloseAll(ctx context.Context, userID uuid.UUID) error
		DeleteExpired(ctx context.Context, limit int) error
	}

	// UserStatisticUseCase - comments interface.
	UserStatisticUseCase interface {
		Execute(ctx context.Context, list []entity.UserActivityLog) error
	}

	// SecureOperationStorage - comments interface.
	SecureOperationStorage interface {
		FetchOne(ctx context.Context, token string) (row entity.SecureOperation, err error)
		Insert(ctx context.Context, row entity.SecureOperation) error
		Update(ctx context.Context, currentToken string, row entity.SecureOperation) error
		UpdateFailedAttempt(ctx context.Context, token string) (attempts uint32, err error)
		Delete(ctx context.Context, token string) error
		DeleteExpired(ctx context.Context, limit int) error
	}

	// SecureOperationLogStorage - comments interface.
	SecureOperationLogStorage interface {
		Insert(ctx context.Context, rows []entity.SecureOperationLog) error
		DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) error
	}

	// OperationHandler - comments interface.
	OperationHandler interface {
		Execute(ctx context.Context, userID uuid.UUID, payload []byte) error
	}

	// User2faStorage - comments interface.
	User2faStorage interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (row entity.Auth2fa, err error)
		InsertOrUpdate(ctx context.Context, row entity.Auth2fa) error
		Delete(ctx context.Context, userID uuid.UUID) error
	}

	// UserActivityStatStorage - comments interface.
	UserActivityStatStorage interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (row entity.UserActivityStat, err error)
		InsertOrUpdate(ctx context.Context, row entity.UserActivityStat) error
		UpdateLastVisited(ctx context.Context, rows []dto.UserActivityLastVisited) error
	}

	// UserActivityLogStorage - comments interface.
	UserActivityLogStorage interface {
		Insert(ctx context.Context, rows []entity.UserActivityLog) error
		DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) error
	}

	// UserStorage - comments interface.
	UserStorage interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (entity.User, error)
		FetchOneByLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (entity.User, error)
		Insert(ctx context.Context, row entity.User) (userID uuid.UUID, err error)
		UpdateEmail(ctx context.Context, userID uuid.UUID, value string) error
		UpdatePhone(ctx context.Context, userID uuid.UUID, value uint64) error
	}

	// UserRealmStorage - comments interface.
	UserRealmStorage interface {
		Fetch(ctx context.Context, userID uuid.UUID) ([]entity.UserRealm, error)
		FetchOne(ctx context.Context, userID uuid.UUID, realm string) (row entity.UserRealm, err error)
		Insert(ctx context.Context, row entity.UserRealm) error
		UpdateKind(ctx context.Context, row entity.UserRealm) error
		Delete(ctx context.Context, userID uuid.UUID, realm string) error
	}

	// OperationEntity - comments interface.
	OperationEntity interface {
		Create(ctx context.Context, opts entity.CreateOperation) (entity.SecureOperation, error)
		GenerateToken() (string, error)
		GenerateSecret(method confirmmethod.Enum) (string, error)
	}

	// FactoryUserConfirm2FA - comments interface.
	FactoryUserConfirm2FA interface {
		CreateByUserLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (dto.User2FA, error)
		CreateByUserID(ctx context.Context, userID uuid.UUID) (dto.User2FA, error)
	}

	// TokenGenerator - comments interface.
	TokenGenerator interface {
		GenToken() (string, error)
		GenTokenLen(length int) (string, error)
	}

	// CodeGenerator - comments interface.
	CodeGenerator interface {
		GenCode() (string, error)
		GenCodeLen(length int) (string, error)
		HashedCode(code string) (string, error)
		CompareCodeAndHash(code, hashedCode string) error
	}

	// TokenIssuer - comments interface.
	TokenIssuer interface {
		Create(realm, userKind, langCode string, userID uuid.UUID) (token dto.AuthToken, err error)
	}

	// ConfirmByAddressCreator - comments interface.
	ConfirmByAddressCreator interface {
		Create(address contactaddress.ContactAddress, confirmCode string) (dto.ConfirmAction, error)
	}
)
