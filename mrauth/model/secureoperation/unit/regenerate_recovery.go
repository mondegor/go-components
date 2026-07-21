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
	// NameConfirmRegenerateRecovery - название операции перевыпуска аварийных кодов пользователя.
	NameConfirmRegenerateRecovery = "confirm.regenerate.recovery"
)

type (
	// RegenerateRecovery - фабрика операции перевыпуска аварийных кодов пользователя.
	RegenerateRecovery struct {
		actionCreator  mrauth.ConfirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewRegenerateRecovery - создаёт объект RegenerateRecovery.
func NewRegenerateRecovery(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action.Option,
) *RegenerateRecovery {
	return &RegenerateRecovery{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - создаёт операцию перевыпуска аварийных кодов для указанного пользователя.
// Требует включённую 2FA: перевыпуск подтверждается email + текущим вторым фактором.
func (o *RegenerateRecovery) Create(user2FA dto.User2FA) (secureoperation.SecureOperation, error) {
	if user2FA.Action2FA.Method == 0 {
		return secureoperation.SecureOperation{}, errors.New("2fa is not enabled")
	}

	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, hashedCode, err := o.codeGenerator.GenCodeWithHash()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	payload, err := BuildRegenerateRecoveryPayload(
		dto.OperationWithUserEmail{
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

	actions = append(actions, user2FA.Action2FA) // 2FA включена (проверено выше) - подтверждение текущим фактором

	return secureoperation.NewOperation(
		operationToken,
		NameConfirmRegenerateRecovery,
		user2FA.ID,
		actions,
		payload,
	)
}
