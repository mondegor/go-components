package unit

import (
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

type (
	// ChangeEmailRequestByRecovery - фабрика операции запроса смены емаила пользователем, утратившим доступ
	// к текущему адресу: цепочка "пароль/TOTP -> аварийный код". Создаёт ту же операцию,
	// что и ChangeEmailRequest (operationtype.ChangeEmail), отличается только составом доказательств,
	// поэтому Opener вытесняет прежнюю смену адреса независимо от выбранной цепочки.
	ChangeEmailRequestByRecovery struct {
		recoveryCreator confirmByRecoveryCreator
		tokenGenerator  mrauth.TokenGenerator
		tokenLength     int
	}
)

// NewChangeEmailRequestByRecovery - создаёт объект ChangeEmailRequestByRecovery.
// Аварийный код предъявляется вместо кода с текущего адреса, поэтому его звено настраивается
// наравне с остальными, а не остаётся на умолчаниях.
func NewChangeEmailRequestByRecovery(
	tokenGenerator mrauth.TokenGenerator,
	tokenLength int,
	confirmByRecoveryOpts ...action.Option,
) *ChangeEmailRequestByRecovery {
	return &ChangeEmailRequestByRecovery{
		recoveryCreator: action.NewConfirmByRecovery(confirmByRecoveryOpts...),
		tokenGenerator:  tokenGenerator,
		tokenLength:     tokenLength,
	}
}

// Create - создаёт операцию запроса смены емаила для пользователя, утратившего доступ к текущему
// адресу: цепочка "пароль/TOTP -> аварийный код", код подтверждения никуда не отправляется.
//
// Подставная цепочка здесь, в отличие от входа, не нужна: метод авторизованный, вызывает его
// сам владелец, и своё состояние 2FA он знает. Поэтому при выключенной 2FA операция честно
// отклоняется: доказательство у аккаунта одно - код на текущий адрес.
func (o *ChangeEmailRequestByRecovery) Create(user2FA dto.User2FA, newEmail contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
	if !newEmail.Is(addresstype.Email) {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("newEmail is not an email address")
	}

	if user2FA.Action2FA.Method == 0 {
		return secureoperation.SecureOperation{}, mrauth.ErrAuth2FAIsDisabled
	}

	operationToken, err := o.tokenGenerator.GenToken(o.tokenLength)
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

	return secureoperation.NewOperation(
		operationToken,
		operationtype.ChangeEmail,
		user2FA.ID,
		[]secureoperation.ConfirmAction{
			newConfirmActionBy2FA(user2FA.Action2FA), o.recoveryCreator.Create(),
		},
		payload,
	)
}
