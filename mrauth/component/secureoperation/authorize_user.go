package secureoperation

import (
	"encoding/json"
	"time"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/component/secureoperation/action"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
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
func (o *AuthorizeUser) Create(user2FA dto.User2FA, realm, langCode string, userLogin contactaddress.ContactAddress) (entity.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	confirmCode, err := o.codeGenerator.GenCode()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	if o.confirmPhoneByEmail && userLogin.Type == addresstype.Phone {
		userLogin = contactaddress.NewEmail(user2FA.Email)
	}

	actions := make([]dto.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(userLogin, confirmCode)
	if err != nil {
		return entity.SecureOperation{}, err
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
		return entity.SecureOperation{}, err
	}

	return entity.SecureOperation{
		Token:             operationToken,
		Name:              NameAuthorizeUser,
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
