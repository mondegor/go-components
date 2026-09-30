package secureoperation

import (
	"context"
	"maps"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// Opener - открытие защищённой операции: единая точка, через которую создаются
	// операции всех типов. Гасит прежние операции того же типа (или той же цепочки) того же
	// пользователя, сохраняет новую и отправляет код её подтверждения.
	Opener struct {
		txManager    mrstorage.DBTxManager
		storage      operationOpenerStorage
		notifierAPI  mrauth.Notifier
		logOperation operationLogger
		errorWrapper errors.Wrapper
	}

	operationOpenerStorage interface {
		DeleteByUserIDAndTypes(ctx context.Context, userID uuid.UUID, types []operationtype.Enum) (deletedTypes []operationtype.Enum, err error)
		Insert(ctx context.Context, row secureoperation.SecureOperation) error
	}

	// operationLogger - best-effort продюсер записей журнала защищённых операций.
	operationLogger interface {
		Log(ctx context.Context, entry entity.SecureOperationLog)
	}
)

// NewOpener - создаёт объект Opener.
func NewOpener(
	txManager mrstorage.DBTxManager,
	storage operationOpenerStorage,
	notifierAPI mrauth.Notifier,
	logOperation operationLogger,
) *Opener {
	return &Opener{
		txManager:    txManager,
		storage:      storage,
		notifierAPI:  notifierAPI,
		logOperation: logOperation,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Open - гасит прежние операции того же типа (или той же цепочки, см. unit.SupersededTypes)
// того же пользователя, сохраняет новую и в той же транзакции отправляет пользователю код
// её подтверждения.
// Вытеснение делает подтверждаемой только последнюю созданную операцию: иначе пользователь
// накапливает несколько операций одного типа и применяет их по очереди, получая повторные
// применения и дубли уведомлений. Операции одной цепочки вытесняют друг друга по той же
// причине: иначе одновременно живут две её ветки.
// Вытеснение выполняется по владельцу и типам операций и realm не учитывает:
// realm хранится в payload операции, непрозрачном для хранилища, и в отбор не входит.
// Это осознанно - открывать операцию одного типа сразу в нескольких realm'ах на практике
// незачем, а если так и произойдёт, действующим останется код последней созданной операции.
// В noteProps передаются дополнительные поля уведомления (например, {"lang": langCode});
// адрес получателя и код подтверждения сервис подставляет сам.
func (o *Opener) Open(
	ctx context.Context,
	actor dto.ActorMeta,
	op secureoperation.SecureOperation,
	noteName string,
	noteProps conv.Group, // OPTIONAL
) error {
	var supersededTypes []operationtype.Enum

	err := o.txManager.Do(ctx, func(ctx context.Context) (err error) {
		// у операции регистрации нового email владельца ещё нет (UserID = uuid.Nil):
		// прежние операции такого пользователя не идентифицировать, гасить нечего
		if op.UserID != uuid.Nil {
			supersededTypes, err = o.storage.DeleteByUserIDAndTypes(ctx, op.UserID, unit.SupersededTypes(op.Type))
			if err != nil {
				return err
			}
		}

		if err := o.storage.Insert(ctx, op); err != nil {
			return err
		}

		return op.NotifyByEmail(
			func(address, confirmCode string) error {
				props := conv.Group{
					"to":          address,
					"confirmCode": confirmCode,
				}
				maps.Copy(props, noteProps)

				return o.notifierAPI.Send(ctx, noteName, props)
			},
		)
	})
	if err != nil {
		return o.errorWrapper.Wrap(err)
	}

	// владелец операции известен - он и фиксируется как посетитель (в анонимных потоках
	// входа и регистрации в actor приходит uuid.Nil, который WithVisitor игнорирует)
	actor = actor.WithVisitor(op.UserID)

	// факт вытеснения фиксируется в журнале как отзыв - по одной записи на каждый
	// вытесненный тип операции
	logged := make(map[operationtype.Enum]struct{}, len(supersededTypes))

	for _, opType := range supersededTypes {
		if _, ok := logged[opType]; ok {
			continue
		}

		logged[opType] = struct{}{}

		o.logOperation.Log(
			ctx,
			actor.NewOperationLog(opType.String(), confirmmethod.Unspecified, logstatus.Revoked, logreason.Superseded),
		)
	}

	// операция создана: фиксируем её инициацию в журнале
	o.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			op.Type.String(), op.FirstActionMethod(), logstatus.Opened, logreason.Unspecified,
		),
	)

	return nil
}
