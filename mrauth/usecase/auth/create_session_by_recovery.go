package auth

import (
	"context"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// CreateSessionByRecovery - инициирует создание сессии пользователя, утратившего доступ
	// к почте: подбирает операцию по realm и создаёт её цепочкой "второй фактор -> аварийный
	// код". Код подтверждения при этом никуда не отправляется.
	CreateSessionByRecovery struct {
		flow createSessionFlow[createSessionByRecoveryOperation]
	}

	// CreateSessionByRecoveryRealm - сопоставление realm с операцией создания сессии
	// по аварийному коду для него.
	CreateSessionByRecoveryRealm struct {
		Name      string
		Operation createSessionByRecoveryOperation
	}

	createSessionByRecoveryOperation interface {
		// Name - имя создаваемой операции; используется для событий журнала, возникающих
		// до её создания (pre-op), чтобы они не разъезжались с именем самой операции.
		Name() string
		Create(user2FA dto.User2FA, realm, langCode string) (secureoperation.SecureOperation, error)
	}
)

// NewCreateSessionByRecovery - создаёт объект CreateSessionByRecovery.
func NewCreateSessionByRecovery(
	opener operationOpener,
	userChecker userLoginChecker,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	logOperation operationLogger,
	allowedRealms []CreateSessionByRecoveryRealm,
) *CreateSessionByRecovery {
	realm2operation := make(map[string]createSessionByRecoveryOperation, len(allowedRealms))
	for _, item := range allowedRealms {
		realm2operation[item.Name] = item.Operation
	}

	return &CreateSessionByRecovery{
		flow: newCreateSessionFlow(
			opener,
			userChecker,
			factoryUser2FAConfirmAction,
			logOperation,
			realm2operation,
			// шаблона нет: первое звено этой цепочки - второй фактор, кода к отправке
			// не возникает вовсе (см. AuthorizeUserByRecovery)
			"",
		),
	}
}

// Execute - создаёт операцию входа для пользователя, утратившего доступ к почте:
// подтверждается вторым фактором и аварийным кодом, письмо не отправляется.
//
// Аккаунту с выключенной 2FA операция создаётся точно так же, с подставным вторым фактором
// (см. unit.AuthorizeUserByRecovery): подтвердить он её не сможет, но и определить
// по ответам, включена ли у аккаунта 2FA, тоже нельзя. Метод гостевой, вызвать его может
// кто угодно, знающий логин, поэтому отказ по состоянию аккаунта здесь недопустим.
func (co *CreateSessionByRecovery) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	realm, langCode string,
	userLogin contactaddress.ContactAddress,
) (secureoperation.SecureOperation, error) {
	return co.flow.execute(
		ctx,
		actor,
		realm,
		langCode,
		userLogin,
		func(opCreator createSessionByRecoveryOperation, user2FA dto.User2FA) (secureoperation.SecureOperation, error) {
			return opCreator.Create(user2FA, realm, langCode)
		},
	)
}
