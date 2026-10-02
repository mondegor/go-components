package secureoperation

import (
	"context"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/dto"
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
			actor dto.ActorMeta,
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
// actor - клиент, подтверждающий операцию, с владельцем операции в UserID (подставляет
// вызывающий, см. dto.ActorMeta.WithUser): по нему проверяется 2FA, а контекст клиента
// попадает в оповещение о расходе аварийного кода. Аварийный код на звене, где он
// не допускается, отклоняется ошибкой mrauth.ErrRecoveryCodeNotAllowed без расхода попытки.
func (o *ConfirmCode) Prepare(
	ctx context.Context,
	actor dto.ActorMeta,
	op secureoperation.SecureOperation,
	confirmCode string,
) (_ secureoperation.SecureOperation, commit func(ctx context.Context) error, err error) {
	if actor.UserID != op.UserID {
		return secureoperation.SecureOperation{}, nil,
			errors.ErrInternalIncorrectInputData.WithDetails("actor is not the operation owner")
	}

	if confirmCode == "" {
		return secureoperation.SecureOperation{}, nil,
			errors.ErrInternalIncorrectInputData.WithDetails("confirmCode is empty")
	}

	confirmed, confirmCodeErr := op.ConfirmAction(
		func(action secureoperation.ConfirmAction) (bool, error) {
			// аварийный код на звене, где он не допускается, отклоняется по одному формату ввода -
			// до сверки и до чтения 2FA, поэтому ответ не зависит от состояния 2FA аккаунта
			if !action.AllowRecovery && action.Method != confirmmethod.Recovery && crypt.IsRecoveryCodeFormat(confirmCode) {
				return false, mrauth.ErrRecoveryCodeNotAllowed
			}

			switch action.Method {
			case confirmmethod.Email, confirmmethod.Phone:
				return o.codeGenerator.CompareSecretAndHash(confirmCode, action.ConfirmCode)
			case confirmmethod.TOTP, confirmmethod.Password, confirmmethod.Recovery:
				ok, factorCommit, err := o.verifier.Verify(ctx, actor, action.Method, action.AllowRecovery, confirmCode)
				if err != nil {
					// 2FA у пользователя нет: либо действие подставное (построено аккаунту с выключенной 2FA),
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
