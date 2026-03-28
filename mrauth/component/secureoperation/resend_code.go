package secureoperation

import (
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ResendCode - comment struct.
	ResendCode struct {
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewResendCode - создаёт объект OperationFactory.
func NewResendCode(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
) *ResendCode {
	return &ResendCode{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
	}
}

// Prepare - comments method.
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
		return secureoperation.SecureOperation{}, err
	}

	if err = op.InitConfirmCode(o.codeGenerator.GenCode); err != nil {
		return secureoperation.SecureOperation{}, err
	}

	return op, nil
}
