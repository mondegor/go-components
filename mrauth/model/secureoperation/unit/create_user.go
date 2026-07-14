package unit

import (
	"encoding/json"

	"github.com/mondegor/go-core/mrtype"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

const (
	// NameConfirmCreateUser - название операции подтверждения создания пользователя.
	NameConfirmCreateUser = "confirm.create.user"
)

type (
	// CreateUser - фабрика операции создания пользователя.
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
	confirmByEmailOpts ...action.Option,
) *CreateUser {
	return &CreateUser{
		realm:          realm,
		userKind:       userKind,
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
		actionCreator:  action.NewConfirmByEmail(confirmByEmailOpts...),
	}
}

// Name - возвращает название создаваемой операции.
func (o *CreateUser) Name() string {
	return NameConfirmCreateUser
}

// Create - создаёт операцию создания пользователя по его email. Если email уже принадлежит
// существующему пользователю с включённым 2FA, то его второй фактор добавляется вторым шагом
// подтверждения - иначе регистрация в новый realm стала бы обходом 2FA.
func (o *CreateUser) Create(
	user2FA dto.User2FA,
	langCode string,
	userEmail contactaddress.ContactAddress,
	registeredIP mrtype.DetailedIP,
) (secureoperation.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, hashedCode, err := o.codeGenerator.GenCodeWithHash()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	payload, err := json.Marshal(
		dto.CreateUserOperation{
			Realm:        o.realm,
			UserKind:     o.userKind,
			LangCode:     langCode,
			Email:        userEmail.Value(),
			RegisteredIP: registeredIP,
		},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	actions := make([]secureoperation.ConfirmAction, 1, 2)

	actions[0], err = o.actionCreator.Create(userEmail, confirmCode, hashedCode)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if user2FA.Action2FA.Method > 0 {
		actions = append(actions, user2FA.Action2FA)
	}

	return secureoperation.NewOperation(
		operationToken,
		NameConfirmCreateUser,
		user2FA.ID,
		actions,
		payload,
	)
}
