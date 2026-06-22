package unit

import (
	"encoding/json"
	"strconv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
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
		actionCreator  mrauth.ConfirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewChangePhone - создаёт объект ChangePhone.
func NewChangePhone(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByPhoneOpts ...action.Option,
) *ChangePhone {
	return &ChangePhone{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByPhone(confirmByPhoneOpts...),
	}
}

// Create - создаёт операцию смены телефона для указанного пользователя.
func (o *ChangePhone) Create(user2FA dto.User2FA, newPhone string) (secureoperation.SecureOperation, error) {
	parsedNewPhone, err := strconv.ParseUint(newPhone, 10, 64)
	if err != nil {
		return secureoperation.SecureOperation{}, contactaddress.ErrPhoneIsInvalid
	}

	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, hashedCode, err := o.codeGenerator.GenCodeWithHash()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	payload, err := json.Marshal(
		dto.ChangePhoneOperation{
			NewPhone: parsedNewPhone,
			Email:    user2FA.Email,
		},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	actions := make([]secureoperation.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(contactaddress.NewPhone(newPhone), confirmCode, hashedCode)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		actions = append(actions, user2FA.Action2FA)
	}

	return secureoperation.NewOperation(
		operationToken,
		NameConfirmChangePhone,
		user2FA.ID,
		actions,
		payload,
	)
}
