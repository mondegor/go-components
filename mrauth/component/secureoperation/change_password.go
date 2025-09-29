package secureoperation

import (
	"encoding/json"
	"time"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/component/secureoperation/action"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
)

const (
	// NameConfirmChangePassword - название операции изменения пароля пользователя.
	NameConfirmChangePassword = "confirm.change.password"
)

type (
	// ChangePassword - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ChangePassword struct {
		actionCreator  mrauth.ConfirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewChangePassword - создаёт объект OperationFactory.
func NewChangePassword(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action.Option,
) *ChangePassword {
	return &ChangePassword{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - comments method.
func (o *ChangePassword) Create(user2FA dto.User2FA, newPassword string) (entity.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	confirmCode, err := o.codeGenerator.GenCode()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	hashedNewPassword, err := o.codeGenerator.HashedCode(newPassword)
	if err != nil {
		return entity.SecureOperation{}, err
	}

	payload, err := json.Marshal(
		dto.ChangePasswordOperation{
			NewPassword:   hashedNewPassword,
			NotifyByEmail: user2FA.Email,
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
		Name:              NameConfirmChangePassword,
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
