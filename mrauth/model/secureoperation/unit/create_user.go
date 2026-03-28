package unit

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	action2 "github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

const (
	// NameConfirmCreateUser - название операции подтверждения создания пользователя.
	NameConfirmCreateUser = "confirm.create.user"
)

type (
	// CreateUser - comment struct.
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
	confirmByEmailOpts ...action2.Option, // TODO: option !!!
) *CreateUser {
	return &CreateUser{
		realm:          realm,
		userKind:       userKind,
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action2.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Create - comments method.
func (o *CreateUser) Create(langCode string, userEmail contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, err := o.codeGenerator.GenCode()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	payload, err := json.Marshal(
		dto.CreateUserOperation{
			Realm:    o.realm,
			UserKind: o.userKind,
			LangCode: langCode,
			Email:    userEmail.Value(),
		},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmAction, err := o.actionCreator.Create(userEmail, confirmCode)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	return secureoperation.NewOperation(
		operationToken,
		NameConfirmCreateUser,
		uuid.Nil,
		[]secureoperation.ConfirmAction{confirmAction},
		payload,
	)
}
