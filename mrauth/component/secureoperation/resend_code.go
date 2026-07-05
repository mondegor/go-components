package secureoperation

import (
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ResendCode - подготовка операции к повторной отправке кода подтверждения.
	ResendCode struct {
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewResendCode - создаёт объект ResendCode.
func NewResendCode(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
) *ResendCode {
	return &ResendCode{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
	}
}

// Prepare - генерирует новый токен и код подтверждения для повторной отправки кода операции.
func (o *ResendCode) Prepare(op secureoperation.SecureOperation) (secureoperation.SecureOperation, error) {
	// if item.Payload["audience"] == "" {
	// 	return 0, errors.New("invalid operation token")
	// }
	//
	// if item.Payload["visitor_id"] == "" {
	// 	return 0, errors.New("invalid operation token")
	// }
	token, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if err = op.ActivateResendCode(token); err != nil {
		if errors.Is(err, secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted) {
			return op, err // WARNING: 'op' используется с этой ошибкой
		}

		return secureoperation.SecureOperation{}, err
	}

	if err = op.InitSendableAction(o.codeGenerator.GenCodeWithHash); err != nil {
		return secureoperation.SecureOperation{}, err
	}

	return op, nil
}
