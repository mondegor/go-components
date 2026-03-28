package unit

import (
	"encoding/json"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	action2 "github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

const (
	// NameConfirmChangeEmail - название операции подтверждения изменения емаила пользователя.
	NameConfirmChangeEmail = "confirm.change.email"
)

type (
	// ChangeEmail - comment struct.
	ChangeEmail struct {
		actionCreator  mrauth.ConfirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewChangeEmail - создаёт объект OperationFactory.
func NewChangeEmail(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action2.Option,
) *ChangeEmail {
	return &ChangeEmail{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action2.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - comments method.
func (o *ChangeEmail) Create(user2FA dto.User2FA, newEmail string) (secureoperation.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, err := o.codeGenerator.GenCode()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	payload, err := json.Marshal(
		dto.ChangeEmailOperation{
			NewEmail:      newEmail,
			NotifyByEmail: user2FA.Email,
		},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	actions := make([]secureoperation.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(contactaddress.NewEmail(newEmail), confirmCode)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		actions = append(actions, user2FA.Action2FA)
	}

	return secureoperation.NewOperation(
		operationToken,
		NameConfirmChangeEmail,
		user2FA.ID,
		actions,
		payload,
	)
}
