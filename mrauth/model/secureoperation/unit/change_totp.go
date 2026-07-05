package unit

import (
	"encoding/json"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

const (
	// NameConfirmChangeTOTP - название операции изменения TOTP пользователя.
	NameConfirmChangeTOTP = "confirm.change.totp"
)

type (
	// ChangeTOTP - фабрика операции смены TOTP пользователя.
	ChangeTOTP struct {
		actionCreator   mrauth.ConfirmByAddressCreator
		tokenGenerator  mrauth.TokenGenerator
		codeGenerator   mrauth.CodeGenerator
		secretGenerator totpSecretGenerator
	}

	// totpSecretGenerator - интерфейс генератора TOTP-секрета для нового аккаунта.
	totpSecretGenerator interface {
		GenerateSecret(accountName string) (secret string, err error)
	}
)

// NewChangeTOTP - создаёт объект ChangeTOTP.
func NewChangeTOTP(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	secretGenerator totpSecretGenerator,
	confirmByEmailOpts ...action.Option,
) *ChangeTOTP {
	return &ChangeTOTP{
		tokenGenerator:  tokenGenerator,
		codeGenerator:   codeGenerator,
		secretGenerator: secretGenerator,
		actionCreator:   action.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - создаёт операцию смены TOTP для указанного пользователя.
func (o *ChangeTOTP) Create(user2FA dto.User2FA) (secureoperation.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, hashedCode, err := o.codeGenerator.GenCodeWithHash()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	secret, err := o.secretGenerator.GenerateSecret(user2FA.Email)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	payload, err := json.Marshal( //nolint:gosec // G117: TOTP-secret намеренно сериализуется в payload операции для последующей привязки.
		dto.ChangeTotpOperation{
			Email:  user2FA.Email,
			Secret: secret,
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
		NameConfirmChangeTOTP,
		user2FA.ID,
		actions,
		payload,
	)
}
