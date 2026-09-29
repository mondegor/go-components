package unit

import (
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

type (
	// AuthorizeUserByRecovery - фабрика операции подтверждения авторизации пользователя,
	// утратившего доступ к почте: цепочка "второй фактор -> аварийный код".
	AuthorizeUserByRecovery struct {
		factor2faCreator    confirmBy2faCreator
		recoveryCreator     confirmByRecoveryCreator
		decoyFactorSelector decoyFactorSelector
		tokenGenerator      mrauth.TokenGenerator
		tokenLength         int
	}

	confirmBy2faCreator interface {
		Create(auth2fa auth2fatype.Enum) (dto.ConfirmAction2FA, error)
	}

	confirmByRecoveryCreator interface {
		Create() secureoperation.ConfirmAction
	}

	decoyFactorSelector interface {
		Select(userID uuid.UUID) auth2fatype.Enum
	}
)

// NewAuthorizeUserByRecovery - создаёт объект AuthorizeUserByRecovery.
func NewAuthorizeUserByRecovery(
	tokenGenerator mrauth.TokenGenerator,
	tokenLength int,
	decoyFactorSelector decoyFactorSelector,
	opts ...AuthorizeUserByRecoveryOption,
) *AuthorizeUserByRecovery {
	var o authorizeUserByRecoveryOptions

	for _, opt := range opts {
		opt(&o)
	}

	return &AuthorizeUserByRecovery{
		factor2faCreator:    action.NewConfirmBy2fa(o.confirmByPassword, o.confirmByTOTP),
		recoveryCreator:     action.NewConfirmByRecovery(o.confirmByRecovery...),
		decoyFactorSelector: decoyFactorSelector,
		tokenGenerator:      tokenGenerator,
		tokenLength:         tokenLength,
	}
}

// Type - возвращает тип создаваемой операции: то же самое, что и у AuthorizeUser.
func (o *AuthorizeUserByRecovery) Type() operationtype.Enum {
	return operationtype.AuthorizeUser
}

// Create - создаёт операцию авторизации пользователя, утратившего доступ к почте:
// цепочка "второй фактор -> аварийный код". Код подтверждения при этом никуда не отправляется.
//
// Аккаунту с выключенной 2FA строится такая же цепочка с подставным типом фактора
// (decoyFactorSelector): доказательств у него нет, подтвердить эту операцию он не сможет,
// но и определить по ответам метода, включена ли у аккаунта 2FA, тоже нельзя - а именно это
// и требуется, ведь метод гостевой и вызвать его может кто угодно, знающий логин.
func (o *AuthorizeUserByRecovery) Create(user2FA dto.User2FA, realm, langCode string) (secureoperation.SecureOperation, error) {
	operationToken, err := o.tokenGenerator.GenToken(o.tokenLength)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	factorAction := user2FA.Action2FA

	if factorAction.Method == 0 {
		factorAction, err = o.factor2faCreator.Create(o.decoyFactorSelector.Select(user2FA.ID))
		if err != nil {
			return secureoperation.SecureOperation{}, err
		}
	}

	payload, err := BuildAuthorizeUserPayload(
		dto.AuthorizeUserOperation{
			Realm:    realm,
			LangCode: langCode,
		},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	// звено второго фактора идёт первым, поэтому AllowRecovery у него быть не должно
	// (инвариант checkInvariants: аварийный код принимается только последним звеном)
	return secureoperation.NewOperation(
		operationToken,
		operationtype.AuthorizeUser,
		user2FA.ID,
		[]secureoperation.ConfirmAction{
			newConfirmActionBy2FA(factorAction), o.recoveryCreator.Create(),
		},
		payload,
	)
}
