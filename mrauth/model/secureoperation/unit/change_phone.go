package unit

import (
	"encoding/json"
	"strconv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	action2 "github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

const (
	// NameConfirmChangePhone - название операции изменения телефона пользователя.
	NameConfirmChangePhone = "confirm.change.phone"
)

type (
	// ChangePhone - comment struct.
	ChangePhone struct {
		actionCreator  mrauth.ConfirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewChangePhone - создаёт объект OperationFactory.
func NewChangePhone(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByPhoneOpts ...action2.Option,
) *ChangePhone {
	return &ChangePhone{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action2.NewConfirmByPhone(confirmByPhoneOpts...),
	}
}

// Create - comments method.
func (o *ChangePhone) Create(user2FA dto.User2FA, newPhone string) (secureoperation.SecureOperation, error) {
	parsedNewPhone, err := strconv.ParseUint(newPhone, 10, 64)
	if err != nil {
		return secureoperation.SecureOperation{}, contactaddress.ErrPhoneIsInvalid
	}

	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, err := o.codeGenerator.GenCode()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	payload, err := json.Marshal(
		dto.ChangePhoneOperation{
			NewPhone:      parsedNewPhone,
			NotifyByEmail: user2FA.Email,
		},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	actions := make([]secureoperation.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(contactaddress.NewPhone(newPhone), confirmCode)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		actions = append(actions, user2FA.Action2FA)
	}

	return secureoperation.New(
		operationToken,
		NameConfirmChangePhone,
		user2FA.ID,
		actions,
		payload,
	)
}
