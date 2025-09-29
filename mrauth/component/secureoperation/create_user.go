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
	// NameConfirmCreateUser - название операции подтверждения создания пользователя.
	NameConfirmCreateUser = "confirm.create.user"
)

type (
	// CreateUser - компонент для извлечения настроек, которые хранятся в хранилище данных.
	CreateUser struct {
		realm          string
		userKind       string
		actionCreator  mrauth.ConfirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewCreateUser - создаёт объект CreateUser.
func NewCreateUser(
	realm string,
	userKind string,
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	confirmByEmailOpts ...action.Option, // TODO: option !!!
) *CreateUser {
	return &CreateUser{
		realm:          realm,
		userKind:       userKind,
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - comments method.
func (o *CreateUser) Create(langCode string, userEmail contactaddress.ContactAddress) (entity.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	confirmCode, err := o.codeGenerator.GenCode()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	payload, err := json.Marshal(
		dto.CreateUserOperation{
			Realm:    o.realm,
			UserKind: o.userKind,
			LangCode: langCode,
			Email:    userEmail.Value,
		},
	)
	if err != nil {
		return entity.SecureOperation{}, err
	}

	confirmAction, err := o.actionCreator.Create(userEmail, confirmCode)
	if err != nil {
		return entity.SecureOperation{}, err
	}

	return entity.SecureOperation{
		Token:             operationToken,
		Name:              NameConfirmCreateUser,
		Actions:           []entity.ConfirmAction{confirmAction},
		RemainingAttempts: confirmAction.MaxAttempts,
		RemainingResends:  confirmAction.MaxResends,
		ResendsAt:         time.Now().Add(confirmAction.MinResendTime).Round(1 * time.Second),
		Payload:           payload,
		Status:            enum.OperationStatusOpened,
		ExpiresAt:         time.Now().Add(confirmAction.Expiry).Round(1 * time.Second),
	}, nil
}
