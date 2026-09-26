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
	// NameConfirmChangePhone - название операции изменения телефона пользователя.
	NameConfirmChangePhone = "confirm.change.phone"
)

type (
	// ChangePhone - фабрика операции смены телефона пользователя.
	ChangePhone struct {
		actionCreator  confirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewChangePhone - создаёт объект ChangePhone.
func NewChangePhone(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action.Option,
) *ChangePhone {
	return &ChangePhone{
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
	}
}

// Create - создаёт операцию смены телефона для указанного пользователя: код подтверждения
// уходит на текущий емаил (операция доказывает владение аккаунтом), затем при включённой
// 2FA - звено второго фактора. Сам новый номер кодом не проверяется: отправка кодов
// на телефон не поддерживается.
func (o *ChangePhone) Create(user2FA dto.User2FA, newPhone contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
	if !newPhone.Is(addresstype.Phone) {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("newPhone is not a phone address")
	}

	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, hashedCode, err := o.codeGenerator.GenCodeWithHash()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	payload, err := BuildChangePhonePayload(
		dto.ChangePhoneOperation{
			NewPhone: newPhone.DigitValue(),
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
		actions = append(actions, newConfirmActionBy2FA(user2FA.Action2FA))
	}

	return secureoperation.NewOperation(
		operationToken,
		NameConfirmChangePhone,
		user2FA.ID,
		actions,
		payload,
	)
}
