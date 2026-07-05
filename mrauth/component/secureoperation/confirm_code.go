package secureoperation

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ConfirmCode - подготовка операции к подтверждению (email/phone/TOTP/password).
	ConfirmCode struct {
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
		verifier       auth2faVerifier
	}

	auth2faVerifier interface {
		Verify(ctx context.Context, userID uuid.UUID, method confirmmethod.Enum, code string) (bool, func(ctx context.Context) error, error)
	}
)

// NewConfirmCode - создаёт объект ConfirmCode.
func NewConfirmCode(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	verifier auth2faVerifier,
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
	if confirmCode == "" {
		return secureoperation.SecureOperation{}, nil, errors.ErrIncorrectInputData.New("confirmCode is empty")
	}

	confirmed, confirmCodeErr := op.ConfirmAction(
		func(action secureoperation.ConfirmAction) (ok bool, err error) {
			switch action.Method {
			case confirmmethod.Email, confirmmethod.Phone:
				return o.codeGenerator.CompareSecretAndHash(confirmCode, action.ConfirmCode)
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
	if confirmCodeErr != nil {
		return op, nil, confirmCodeErr // WARNING: 'op' используется с этой ошибкой
	}

	if confirmed {
		return op, commit, nil
	}

	// ВНИМАНИЕ: в эту часть кода можно попасть ТОЛЬКО после успешного sendable-действия (email/phone),
	// у которого commit всегда nil (при этом, данная операция подтверждена ещё НЕ полностью).
	// Успешное 2FA-действие (TOTP/password) сюда попасть не может: по инварианту checkInvariants
	// оно всегда последнее в цепочке, поэтому его успех сразу даёт confirmed == true (ветка выше),
	// и его commit возвращается вызывающему для расхода аварийного кода в той же транзакции.
	// Значит, здесь расходовать нечего и возврат commit == nil корректен - аварийный код не теряется.

	// для следующего (sendable) действия генерится новый токен и код подтверждения
	token, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, nil, err
	}

	if err = op.ActivateConfirmation(token); err != nil {
		return secureoperation.SecureOperation{}, nil, err
	}

	if err = op.InitSendableAction(o.codeGenerator.GenCodeWithHash); err != nil {
		return secureoperation.SecureOperation{}, nil, err
	}

	return op, nil, nil
}
