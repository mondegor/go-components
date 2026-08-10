package unit

import (
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
	// AuthorizeUser - фабрика операции подтверждения авторизации пользователя.
	AuthorizeUser struct {
		actionCreator       confirmByAddressCreator
		tokenGenerator      mrauth.TokenGenerator
		codeGenerator       mrauth.CodeGenerator
		confirmPhoneByEmail bool
	}

	confirmByAddressCreator interface {
		Create(address contactaddress.ContactAddress, confirmCode, hashedConfirmCode string) (secureoperation.ConfirmAction, error)
	}
)

// NewAuthorizeUser - создаёт объект AuthorizeUser.
func NewAuthorizeUser(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	opts ...AuthorizeUserOption,
) *AuthorizeUser {
	o := authorizeUserOptions{
		confirmPhoneByEmail: defaultConfirmPhoneByEmail,
	}

	for _, opt := range opts {
		opt(&o)
	}

	return &AuthorizeUser{
		actionCreator:       action.NewConfirmByAddress(o.confirmByEmail, o.confirmByPhone),
		tokenGenerator:      tokenGenerator,
		codeGenerator:       codeGenerator,
		confirmPhoneByEmail: o.confirmPhoneByEmail,
	}
}

// Name - возвращает название создаваемой операции.
func (o *AuthorizeUser) Name() string {
	return NameAuthorizeUser
}

// Create - создаёт операцию авторизации пользователя по его логину (email/телефон):
// цепочка "код с емаила/телефона -> второй фактор". Вместо второго фактора допускается
// предъявить аварийный код, поэтому цепочка покрывает сразу две комбинации из трёх;
// третью ("второй фактор + аварийный код") строит фабрика AuthorizeUserByRecovery.
func (o *AuthorizeUser) Create(user2FA dto.User2FA, realm, langCode string, userLogin contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, hashedCode, err := o.codeGenerator.GenCodeWithHash()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if o.confirmPhoneByEmail && userLogin.Is(addresstype.Phone) {
		userLogin = contactaddress.NewEmail(user2FA.Email)
	}

	actions := make([]secureoperation.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(userLogin, confirmCode, hashedCode)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		factorAction := newConfirmActionBy2FA(user2FA.Action2FA)

		// аварийный код принимается вместо второго фактора: это завершающее действие цепочки,
		// поэтому его успех сразу означает, что комбинация доказательств принята целиком
		factorAction.AllowRecovery = true

		actions = append(actions, factorAction)
	}

	payload, err := BuildAuthorizeUserPayload(
		dto.AuthorizeUserOperation{
			Realm:    realm,
			LangCode: langCode, // TODO: only for !o.confirmPhoneByEmail or if new environment
			// Email:     user2FA.Email, // TODO: only for !o.confirmPhoneByEmail or if new environment
		},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	return secureoperation.NewOperation(
		operationToken,
		NameAuthorizeUser,
		user2FA.ID,
		actions,
		payload,
	)
}
