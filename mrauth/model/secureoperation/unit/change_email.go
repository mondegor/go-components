package unit

import (
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

const (
	// NameConfirmChangeEmail - название операции подтверждения изменения емаила пользователя.
	NameConfirmChangeEmail = "confirm.change.email"
)

type (
	// ChangeEmail - фабрика операции смены email пользователя.
	ChangeEmail struct {
		actionCreator  confirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewChangeEmail - создаёт объект ChangeEmail.
func NewChangeEmail(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action.Option,
) *ChangeEmail {
	return &ChangeEmail{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - создаёт операцию смены email для указанного пользователя.
func (o *ChangeEmail) Create(user2FA dto.User2FA, newEmail contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
	if !newEmail.Is(addresstype.Email) {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("newEmail is not an email address")
	}

	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, hashedCode, err := o.codeGenerator.GenCodeWithHash()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	payload, err := BuildChangeEmailPayload(
		dto.ChangeEmailOperation{
			NewEmail: newEmail.Value(),
			Email:    user2FA.Email,
		},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	actions := make([]secureoperation.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(newEmail, confirmCode, hashedCode)
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
