package secureoperation

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/component/secureoperation/action"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
)

const (
	// NameConfirmChangePhone - название операции изменения телефона пользователя.
	NameConfirmChangePhone = "confirm.change.phone"
)

type (
	// ChangePhone - компонент для извлечения настроек, которые хранятся в хранилище данных.
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
	confirmByPhoneOpts ...action.Option,
) *ChangePhone {
	return &ChangePhone{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByPhone(confirmByPhoneOpts...),
	}
}

// Create - comments method.
func (o *ChangePhone) Create(user2FA dto.User2FA, newPhone string) (entity.SecureOperation, error) {
	parsedNewPhone, err := strconv.ParseUint(newPhone, 10, 64)
	if err != nil {
		return entity.SecureOperation{}, contactaddress.ErrPhoneIsInvalid.New()
	}

	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	confirmCode, err := o.codeGenerator.GenCode()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	payload, err := json.Marshal(
		dto.ChangePhoneOperation{
			NewPhone:      parsedNewPhone,
			NotifyByEmail: user2FA.Email,
		},
	)
	if err != nil {
		return entity.SecureOperation{}, err
	}

	actions := make([]entity.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(contactaddress.NewPhone(newPhone), confirmCode)
	if err != nil {
		return entity.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		actions = append(actions, user2FA.Action2FA)
	}

	return entity.SecureOperation{
		Token:             operationToken,
		Name:              NameConfirmChangePhone,
		UserID:            user2FA.ID,
		Actions:           actions,
		RemainingAttempts: actions[0].MaxAttempts,
		RemainingResends:  actions[0].MaxResends,
		ResendsAt:         time.Now().Add(actions[0].MinResendTime).Round(1 * time.Second),
		Payload:           payload,
		Status:            enum.OperationStatusOpened,
		ExpiresAt:         time.Now().Add(actions[0].Expiry).Round(1 * time.Second),
	}, nil
}
