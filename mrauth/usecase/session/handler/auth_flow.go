package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
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
		PrepareAuthorization(ctx context.Context, actor dto.ActorMeta, in dto.AuthorizeUserOperation) (dto.UserScopes, func(context.Context), error)
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
// Вместе со scopes возвращается отложенный callback отправки login-alert'а с контекстом клиента actor.
// actor - анонимный клиент либо сам владелец операции.
func (uc *AuthFlow) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	op secureoperation.SecureOperation,
) (scopes dto.UserScopes, notifyAuthSuccess func(context.Context), err error) {
	if actor.UserID != uuid.Nil && actor.UserID != op.UserID {
		return dto.UserScopes{}, nil, errors.ErrInternalIncorrectInputData.WithDetails("actor is not the operation owner")
	}

	var authIn dto.AuthorizeUserOperation

	switch op.Type {
	case operationtype.CreateUser:
		var createIn dto.CreateUserOperation

		if createIn, err = unit.ParseCreateUserPayload(op.Payload); err != nil {
			return dto.UserScopes{}, nil, err
		}

		op.UserID, err = uc.service.ResolveUser(ctx, op.UserID, createIn)
		if err != nil {
			return dto.UserScopes{}, nil, err
		}

		authIn = dto.AuthorizeUserOperation{
			Realm:    createIn.Realm,
			LangCode: createIn.LangCode,
		}
	case operationtype.AuthorizeUser:
		if authIn, err = unit.ParseAuthorizeUserPayload(op.Payload); err != nil {
			return dto.UserScopes{}, nil, err
		}
	default:
		return dto.UserScopes{}, nil, errors.ErrInternalIncorrectInputData.WithDetails("operation type is incorrect", "type", op.Type)
	}

	// пользователь известен (из операции или только что разрешён) - он и фиксируется как посетитель
	return uc.service.PrepareAuthorization(ctx, actor.WithUser(op.UserID), authIn)
}
