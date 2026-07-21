package unit

import (
	"errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

const (
	// NameConfirmDisable2FA - название операции подтверждения отключения 2FA пользователя.
	NameConfirmDisable2FA = "confirm.disable.2fa"
)

type (
	// Disable2FA - фабрика операции отключения 2FA пользователя.
	Disable2FA struct {
		actionCreator  mrauth.ConfirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewDisable2FA - создаёт объект Disable2FA.
func NewDisable2FA(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action.Option,
) *Disable2FA {
	return &Disable2FA{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - создаёт операцию отключения 2FA для указанного пользователя.
func (o *Disable2FA) Create(user2FA dto.User2FA) (secureoperation.SecureOperation, error) {
	if user2FA.Action2FA.Method == 0 {
		return secureoperation.SecureOperation{}, errors.New("2fa already disabled")
	}

	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, hashedCode, err := o.codeGenerator.GenCodeWithHash()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	payload, err := BuildDisable2FAPayload(
		dto.Disable2FAOperation{
			Email: user2FA.Email,
		},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	actions := make([]secureoperation.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(contactaddress.NewEmail(user2FA.Email), confirmCode, hashedCode)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		actions = append(actions, user2FA.Action2FA)
	}

	return secureoperation.NewOperation(
		operationToken,
		NameConfirmDisable2FA,
		user2FA.ID,
		actions,
		payload,
	)
}
