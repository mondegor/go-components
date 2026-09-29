package unit

import (
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

type (
	// Disable2FA - фабрика операции отключения 2FA пользователя.
	Disable2FA struct {
		actionCreator  confirmByAddressCreator
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
// Требует включённую 2FA: снятие подтверждается email-кодом и текущим вторым фактором,
// вместо которого допустим аварийный код.
func (o *Disable2FA) Create(user2FA dto.User2FA) (secureoperation.SecureOperation, error) {
	if user2FA.Action2FA.Method == 0 {
		return secureoperation.SecureOperation{}, mrauth.ErrAuth2FAIsDisabled
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

	factorAction := newConfirmActionBy2FA(user2FA.Action2FA)

	// снятие 2FA обязательно подтверждается email-кодом, поэтому аварийный код допустим
	// только вторым действием: комбинация "пароль/TOTP + аварийный код" отклоняется
	factorAction.AllowRecovery = true

	actions = append(actions, factorAction)

	return secureoperation.NewOperation(
		operationToken,
		operationtype.Disable2FA,
		user2FA.ID,
		actions,
		payload,
	)
}
