package mrauth

import (
	"context"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// OperationUseCase - comments interface.
	OperationUseCase interface {
		ConfirmAction(ctx context.Context, operationToken, secret string) (secureoperation.SecureOperation, error)
		ResendCode(ctx context.Context, operationToken string) (secureoperation.SecureOperation, error)
		Revoke(ctx context.Context, operationToken string) error
	}

	// AuthTokenFetcher - предоставляет доступ к хранилищу сообщений.
	AuthTokenFetcher interface {
		FetchOne(ctx context.Context, accessToken string) (dto.UserScopes, error)
	}

	// UserStatisticUseCase - comments interface.
	UserStatisticUseCase interface {
		Execute(ctx context.Context, list []dto.UserActivityLogMessage) error
	}

	// OperationHandler - comments interface.
	OperationHandler interface {
		Execute(ctx context.Context, userID uuid.UUID, payload []byte) error
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
		Create(userScopes dto.UserScopes) (token dto.AuthToken, err error)
	}

	// ConfirmByAddressCreator - comments interface.
	ConfirmByAddressCreator interface {
		Create(address contactaddress.ContactAddress, confirmCode string) (secureoperation.ConfirmAction, error)
	}
)
