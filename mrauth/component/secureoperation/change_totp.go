package secureoperation

import (
	"encoding/json"
	"time"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/component/secureoperation/action"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

const (
	// NameConfirmChangeTOTP - название операции изменения TOTP пользователя.
	NameConfirmChangeTOTP = "confirm.change.totp"
)

type (
	// ChangeTOTP - comment struct.
	ChangeTOTP struct {
		actionCreator  mrauth.ConfirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewChangeTOTP - создаёт объект OperationFactory.
func NewChangeTOTP(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action.Option,
) *ChangeTOTP {
	return &ChangeTOTP{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - comments method.
func (o *ChangeTOTP) Create(user2FA dto.User2FA) (entity.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	confirmCode, err := o.codeGenerator.GenCode()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	payload, err := json.Marshal(
		dto.ChangeTotpOperation{
			Email: user2FA.Email, // UserLogin and Email
		},
	)
	if err != nil {
		return entity.SecureOperation{}, err
	}

	actions := make([]dto.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(contactaddress.NewEmail(user2FA.Email), confirmCode)
	if err != nil {
		return entity.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		actions = append(actions, user2FA.Action2FA)
	}

	return entity.SecureOperation{
		Token:             operationToken,
		Name:              NameConfirmChangeTOTP,
		UserID:            user2FA.ID,
		Actions:           actions,
		RemainingAttempts: actions[0].MaxAttempts,
		RemainingResends:  actions[0].MaxResends,
		ResendsAt:         time.Now().Add(actions[0].MinResendTime).Round(1 * time.Second),
		Payload:           payload,
		Status:            operationstatus.Opened,
		ExpiresAt:         time.Now().Add(actions[0].Expiry).Round(1 * time.Second),
	}, nil
}
