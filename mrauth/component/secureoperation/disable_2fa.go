package secureoperation

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/component/secureoperation/action"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
)

const (
	// NameConfirmDisable2FA - название операции подтверждения отключения 2FA пользователя.
	NameConfirmDisable2FA = "confirm.disable.2fa"
)

type (
	// Disable2FA - компонент для извлечения настроек, которые хранятся в хранилище данных.
	Disable2FA struct {
		actionCreator  mrauth.ConfirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewDisable2FA - создаёт объект OperationFactory.
func NewDisable2FA(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action.Option, // TODO: option !!!
) *Disable2FA {
	return &Disable2FA{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - comments method.
func (o *Disable2FA) Create(user2FA dto.User2FA) (entity.SecureOperation, error) {
	if user2FA.Action2FA.Method == 0 {
		return entity.SecureOperation{}, errors.New("2fa already disabled") // already disabled !!!!!!!!!!!!!!!
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
		dto.Disable2faOperation{
			Email: user2FA.Email,
		},
	)
	if err != nil {
		return entity.SecureOperation{}, err
	}

	actions := make([]entity.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(contactaddress.NewEmail(user2FA.Email), confirmCode)
	if err != nil {
		return entity.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		actions = append(actions, user2FA.Action2FA)
	}

	return entity.SecureOperation{
		Token:             operationToken,
		Name:              NameConfirmDisable2FA,
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
