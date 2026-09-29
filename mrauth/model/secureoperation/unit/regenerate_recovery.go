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
	// RegenerateRecovery - фабрика операции перевыпуска аварийных кодов пользователя.
	RegenerateRecovery struct {
		actionCreator  confirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		tokenLength    int
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewRegenerateRecovery - создаёт объект RegenerateRecovery.
func NewRegenerateRecovery(
	tokenGenerator mrauth.TokenGenerator,
	tokenLength int,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action.Option,
) *RegenerateRecovery {
	return &RegenerateRecovery{
		tokenGenerator: tokenGenerator,
		tokenLength:    tokenLength,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - создаёт операцию перевыпуска аварийных кодов для указанного пользователя.
// Требует включённую 2FA: перевыпуск подтверждается email + текущим вторым фактором.
func (o *RegenerateRecovery) Create(user2FA dto.User2FA) (secureoperation.SecureOperation, error) {
	if user2FA.Action2FA.Method == 0 {
		return secureoperation.SecureOperation{}, mrauth.ErrAuth2FAIsDisabled
	}

	operationToken, err := o.tokenGenerator.GenToken(o.tokenLength)
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

	actions[0], err = o.actionCreator.Create(contactaddress.NewEmail(user2FA.Email))
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	// AllowRecovery не выставляется ни одному действию: перевыпуск принимает единственную
	// комбинацию "email-код + пароль/TOTP", любая с аварийным кодом отклоняется
	actions = append(actions, newConfirmActionBy2FA(user2FA.Action2FA)) // 2FA включена (проверено выше) - подтверждение текущим фактором

	return newSendableOperation(
		o.codeGenerator,
		operationToken,
		operationtype.RegenerateRecovery,
		user2FA.ID,
		actions,
		payload,
	)
}
