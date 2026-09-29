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
	// ChangeEmailRequest - фабрика операции запроса смены емаила пользователя (первый шаг):
	// владение аккаунтом доказывается кодом с текущего адреса и, при включённой 2FA, вторым фактором.
	ChangeEmailRequest struct {
		actionCreator  confirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewChangeEmailRequest - создаёт объект ChangeEmailRequest.
func NewChangeEmailRequest(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action.Option,
) *ChangeEmailRequest {
	return &ChangeEmailRequest{
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
	}
}

// Create - создаёт операцию запроса смены емаила для указанного пользователя; сам емаил
// меняет операция второго шага (operationtype.ChangeEmailConfirm), которую открывает применение этой.
// Утратившему доступ к почте предназначена фабрика ChangeEmailRequestByRecovery.
func (o *ChangeEmailRequest) Create(user2FA dto.User2FA, newEmail contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
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

	actions[0], err = o.actionCreator.Create(contactaddress.NewEmail(user2FA.Email), confirmCode, hashedCode)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		// аварийный код этой цепочке недопустим ни на одном звене: смена адреса обязательно
		// подтверждается паролем/TOTP, поэтому комбинация "email-код + аварийный код"
		// отклоняется. Комбинацию "пароль/TOTP + аварийный код" строит ChangeEmailRequestByRecovery
		actions = append(actions, newConfirmActionBy2FA(user2FA.Action2FA))
	}

	return secureoperation.NewOperation(
		operationToken,
		operationtype.ChangeEmail,
		user2FA.ID,
		actions,
		payload,
	)
}
