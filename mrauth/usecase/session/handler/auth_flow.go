package handler

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// AuthFlow - объединённый обработчик подтверждённой операции создания и авторизации пользователя.
	AuthFlow struct {
		service authUserService
	}

	authUserService interface {
		ResolveUser(ctx context.Context, userID uuid.UUID, in dto.CreateUserOperation) (resolvedUserID uuid.UUID, err error)
		PrepareAuthorization(ctx context.Context, userID uuid.UUID, in dto.AuthorizeUserOperation) (dto.UserScopes, func(context.Context), error)
	}
)

// NewAuthFlow - создаёт объект AuthFlow.
func NewAuthFlow(service authUserService) *AuthFlow {
	return &AuthFlow{
		service: service,
	}
}

// Execute - выполняет подготовку scopes пользователя по подтверждённой операции.
// Для операции создания пользователя сначала создаёт его (вариант 1) либо распознаёт,
// что он уже зарегистрирован (вариант 2), после чего операция трактуется как авторизация.
// Для операции авторизации (вариант 3) сразу выполняется подготовка к авторизации.
//
// Вместе со scopes возвращается отложенный callback отправки login-alert'а.
func (uc *AuthFlow) Execute(
	ctx context.Context,
	op secureoperation.SecureOperation,
) (scopes dto.UserScopes, notifyAuthSuccess func(context.Context), err error) {
	authIn := dto.AuthorizeUserOperation{}

	switch op.Name {
	case unit.NameConfirmCreateUser:
		createIn := dto.CreateUserOperation{}

		if err = json.Unmarshal(op.Payload, &createIn); err != nil {
			return dto.UserScopes{}, nil, errors.ErrInternalIncorrectInputData.WithError(err, "AuthFlow", "operation_name", op.Name, "user_id", op.UserID)
		}

		op.UserID, err = uc.service.ResolveUser(ctx, op.UserID, createIn)
		if err != nil {
			return dto.UserScopes{}, nil, err
		}

		authIn = dto.AuthorizeUserOperation{
			Realm:    createIn.Realm,
			LangCode: createIn.LangCode,
		}
	case unit.NameAuthorizeUser:
		if err = json.Unmarshal(op.Payload, &authIn); err != nil {
			return dto.UserScopes{}, nil, errors.ErrInternalIncorrectInputData.WithError(err, "AuthFlow", "payload", op.Payload)
		}
	default:
		return dto.UserScopes{}, nil, errors.ErrInternalIncorrectInputData.WithDetails("operation name is incorrect", "name", op.Name)
	}

	return uc.service.PrepareAuthorization(ctx, op.UserID, authIn)
}
