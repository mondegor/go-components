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
		Verify(
			ctx context.Context,
			userID uuid.UUID,
			method confirmmethod.Enum,
			allowRecovery bool,
			code string,
		) (ok bool, commit func(ctx context.Context) error, err error)
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

// Prepare - проверяет текущее действие операции; для TOTP/password и аварийного кода
// использует верификатор. Возвращает commit расхода второго фактора (продвинутый TOTP-шаг
// или погашенный аварийный код), который должен быть вызван в транзакции подтверждения.
func (o *ConfirmCode) Prepare(
	ctx context.Context,
	op secureoperation.SecureOperation,
	confirmCode string,
) (_ secureoperation.SecureOperation, commit func(ctx context.Context) error, err error) {
	if confirmCode == "" {
		return secureoperation.SecureOperation{}, nil,
			errors.ErrInternalIncorrectInputData.WithDetails("confirmCode is empty")
	}

	confirmed, confirmCodeErr := op.ConfirmAction(
		func(action secureoperation.ConfirmAction) (bool, error) {
			switch action.Method {
			case confirmmethod.Email, confirmmethod.Phone:
				return o.codeGenerator.CompareSecretAndHash(confirmCode, action.ConfirmCode)
			case confirmmethod.TOTP, confirmmethod.Password, confirmmethod.Recovery:
				ok, factorCommit, err := o.verifier.Verify(ctx, op.UserID, action.Method, action.AllowRecovery, confirmCode)
				if err != nil {
					// строки 2FA нет: либо действие подставное (построено аккаунту с выключенной 2FA),
					// либо 2FA сняли уже после создания операции. Отдельным кодом ответа эти случаи
					// не отражаются вовсе: метод гостевой, и любой отличающийся ответ читался бы как
					// состояние 2FA аккаунта - ровно то, что подстановка скрывает. Поэтому отказ
					// выглядит как неверно введённое доказательство
					if errors.Is(err, mrauth.ErrAuth2FAIsDisabled) {
						return false, nil
					}

					return false, err
				}

				commit = factorCommit

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

	// сюда попадает непоследнее действие цепочки: операция подтверждена ещё НЕ полностью.
	// Аварийный код израсходовать здесь нельзя - по инварианту checkInvariants он принимается
	// только последним действием, поэтому его успех сразу даёт confirmed == true (ветка выше).
	// А вот commit непустым быть может: в цепочке "2FA -> аварийный код" первым идёт TOTP,
	// и его продвинутый шаг обязан попасть в ту же транзакцию подтверждения.

	// для следующего действия генерится новый токен, а если оно sendable - ещё и код подтверждения
	token, err := o.tokenGenerator.GenToken(len(op.Token))
	if err != nil {
		return secureoperation.SecureOperation{}, nil, err
	}

	if err = op.ActivateConfirmation(token); err != nil {
		return secureoperation.SecureOperation{}, nil, err
	}

	if err = op.InitSendableAction(o.codeGenerator.GenCodeWithHash); err != nil {
		return secureoperation.SecureOperation{}, nil, err
	}

	return op, commit, nil
}
