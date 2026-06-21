package secureoperation

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ConfirmCode - подготовка операции к подтверждению (email/phone/TOTP/password).
	ConfirmCode struct {
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
		verifier       secondFactorVerifier
	}

	secondFactorVerifier interface {
		Verify(ctx context.Context, userID uuid.UUID, method confirmmethod.Enum, code string) (bool, func(ctx context.Context) error, error)
	}
)

// NewConfirmCode - создаёт объект ConfirmCode.
func NewConfirmCode(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	verifier secondFactorVerifier,
) *ConfirmCode {
	return &ConfirmCode{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		verifier:       verifier,
	}
}

// Prepare - проверяет текущее действие операции; для TOTP/password использует верификатор.
// Возвращает commit для расходования аварийного кода (если он был использован),
// который должен быть вызван в транзакции подтверждения.
func (o *ConfirmCode) Prepare(
	ctx context.Context,
	op secureoperation.SecureOperation,
	confirmCode string,
) (_ secureoperation.SecureOperation, commit func(ctx context.Context) error, err error) {
	confirmed, err := op.ConfirmAction(
		func(action secureoperation.ConfirmAction) (ok bool, err error) {
			switch action.Method {
			case confirmmethod.Email, confirmmethod.Phone:
				return action.ConfirmCode == confirmCode, nil
			case confirmmethod.TOTP, confirmmethod.Password:
				ok, commit, err = o.verifier.Verify(ctx, op.UserID, action.Method, confirmCode)
				if err != nil {
					return false, err
				}

				return ok, nil
			default:
				return false, errors.NewInternalError("ConfirmMethod is not supported", "method", action.Method)
			}
		},
	)
	if err != nil {
		return op, nil, err // WARNING: 'op' используется с этой ошибкой
	}

	if confirmed {
		return op, commit, nil
	}

	// для следующего (sendable) действия генерится новый токен и код подтверждения
	token, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, nil, err
	}

	if err = op.ActivateConfirmation(token); err != nil {
		return secureoperation.SecureOperation{}, nil, err
	}

	if err = op.InitSendableAction(o.codeGenerator.GenCode); err != nil {
		return secureoperation.SecureOperation{}, nil, err
	}

	return op, commit, nil
}
