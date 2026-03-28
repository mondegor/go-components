package secureoperation

import (
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ConfirmCode - comment struct.
	ConfirmCode struct {
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewConfirmCode - создаёт объект OperationFactory.
func NewConfirmCode(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
) *ConfirmCode {
	return &ConfirmCode{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
	}
}

// Prepare - comments method.
func (o *ConfirmCode) Prepare(op secureoperation.SecureOperation, confirmCode string) (secureoperation.SecureOperation, error) {
	confirmed, err := op.ConfirmAction(
		func(_ confirmmethod.Enum, code string) bool {
			// TODO:
			// для метода password нужно сравнивать с hash пароля
			// для метода totp нужно сравнивать с кодом в приложении
			return code == confirmCode
		},
	)
	if err != nil {
		return op, err // WARNING: 'op' используется с этой ошибкой
	}

	if confirmed {
		return op, nil
	}

	// для нового подтверждения генерится новый токен
	token, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if err = op.ActivateConfirmation(token); err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if err = op.InitConfirmCode(o.codeGenerator.GenCode); err != nil {
		return secureoperation.SecureOperation{}, err
	}

	return op, nil
}
