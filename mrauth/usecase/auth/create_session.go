package auth

import (
	"context"

	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// CreateSession - инициирует создание сессии пользователя: подбирает операцию по
	// realm, создаёт её и отправляет код подтверждения по логину пользователя.
	CreateSession struct {
		flow createSessionFlow[createSessionOperation]
	}

	// CreateSessionRealm - сопоставление realm с операцией создания сессии для него.
	CreateSessionRealm struct {
		Name      string
		Operation createSessionOperation
	}

	// operationOpener - открывает созданную операцию: гасит прежние операции того же
	// типа, сохраняет новую, отправляет код подтверждения и пишет журнал.
	operationOpener interface {
		Open(
			ctx context.Context,
			actor dto.ActorMeta,
			op secureoperation.SecureOperation,
			noteName string,
			noteProps conv.Group,
		) error
	}

	userLoginChecker interface {
		CheckAvailabilityRealm(ctx context.Context, realm string, userLogin contactaddress.ContactAddress) error
	}

	createSessionOperation interface {
		// Type - тип создаваемой операции; используется для событий журнала, возникающих
		// до её создания (pre-op), чтобы они не разъезжались с типом самой операции.
		Type() operationtype.Enum
		Create(user2FA dto.User2FA, realm, langCode string, address contactaddress.ContactAddress) (secureoperation.SecureOperation, error)
	}
)

// NewCreateSession - создаёт объект CreateSession.
func NewCreateSession(
	opener operationOpener,
	userChecker userLoginChecker,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	logOperation operationLogger,
	allowedRealms []CreateSessionRealm,
) *CreateSession {
	realm2operation := make(map[string]createSessionOperation, len(allowedRealms))
	for _, item := range allowedRealms {
		realm2operation[item.Name] = item.Operation
	}

	return &CreateSession{
		flow: newCreateSessionFlow(
			opener,
			userChecker,
			factoryUser2FAConfirmAction,
			logOperation,
			realm2operation,
			"confirm.create.session.by.email",
		),
	}
}

// Execute - проверяет логин пользователя в рамках realm, создаёт операцию создания
// сессии и в той же транзакции отправляет пользователю код её подтверждения.
func (co *CreateSession) Execute(
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
		func(opCreator createSessionOperation, user2FA dto.User2FA) (secureoperation.SecureOperation, error) {
			return opCreator.Create(user2FA, realm, langCode, userLogin)
		},
	)
}
