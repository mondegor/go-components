package unit

import (
	"encoding/json"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

const (
	// NameAuthorizeUser - название операции подтверждения авторизации пользователя.
	NameAuthorizeUser = "confirm.authorize.user"
)

type (
	// AuthorizeUser - comment struct.
	AuthorizeUser struct {
		actionCreator       mrauth.ConfirmByAddressCreator
		tokenGenerator      mrauth.TokenGenerator
		codeGenerator       mrauth.CodeGenerator
		confirmPhoneByEmail bool
	}
)

// NewAuthorizeUser - создаёт объект OperationFactory.
func NewAuthorizeUser(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	opts ...AuthorizeUserOption,
) *AuthorizeUser {
	o := authorizeUserOptions{
		authorizer: &AuthorizeUser{
			tokenGenerator:      tokenGenerator,
			codeGenerator:       codeGenerator,
			confirmPhoneByEmail: defaultConfirmPhoneByEmail,
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	o.authorizer.actionCreator = action.NewConfirmByAddress(
		o.confirmByEmail,
		o.confirmByPhone,
	)

	return o.authorizer
}

// Create - comments method.
func (o *AuthorizeUser) Create(user2FA dto.User2FA, realm, langCode string, userLogin contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, err := o.codeGenerator.GenCode()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if o.confirmPhoneByEmail && userLogin.Is(addresstype.Phone) {
		userLogin = contactaddress.NewEmail(user2FA.Email)
	}

	actions := make([]secureoperation.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(userLogin, confirmCode)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		actions = append(actions, user2FA.Action2FA)
	}

	payload, err := json.Marshal(
		dto.AuthorizeUserOperation{
			Realm:    realm,
			LangCode: langCode, // TODO: only for !o.confirmPhoneByEmail or if new environment
			// Email:     user2FA.Email, // TODO: only for !o.confirmPhoneByEmail or if new environment
		},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	return secureoperation.New(
		operationToken,
		NameAuthorizeUser,
		user2FA.ID,
		actions,
		payload,
	)
}
