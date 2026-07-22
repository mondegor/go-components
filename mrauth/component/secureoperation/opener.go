package secureoperation

import (
	"context"
	"maps"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// Opener - открытие защищённой операции: единая точка, через которую создаются
	// операции всех типов. Гасит прежние незавершённые операции того же типа того же
	// пользователя, сохраняет новую и отправляет код её подтверждения.
	Opener struct {
		txManager    mrstorage.DBTxManager
		storage      operationOpenerStorage
		notifierAPI  mrnotifier.NoteProducer
		logOperation operationLogger
		errorWrapper errors.Wrapper
	}

	operationOpenerStorage interface {
		DeleteByUserIDAndName(ctx context.Context, userID uuid.UUID, name string) error
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
	notifierAPI mrnotifier.NoteProducer,
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

// Open - гасит прежние операции того же типа того же пользователя, сохраняет новую
// и в той же транзакции отправляет пользователю код её подтверждения.
// Вытеснение делает подтверждаемой только последнюю созданную операцию: иначе пользователь
// накапливает несколько операций одного типа и применяет их по очереди, получая повторные
// применения и дубли уведомлений.
// Вытеснение выполняется по паре (владелец, имя операции) и realm не учитывает:
// realm хранится в payload операции и в предикат попасть не может. Это осознанно -
// открывать операцию одного типа сразу в нескольких realm'ах на практике незачем,
// а если так и произойдёт, действующим останется код последней созданной операции.
// В noteProps передаются дополнительные поля уведомления (например, {"lang": langCode});
// адрес получателя и код подтверждения компонент подставляет сам.
func (o *Opener) Open(
	ctx context.Context,
	actor dto.ActorMeta,
	op secureoperation.SecureOperation,
	noteName string,
	noteProps conv.Group, // OPTIONAL
) error {
	var superseded bool

	err := o.txManager.Do(ctx, func(ctx context.Context) error {
		// у операции регистрации нового email владельца ещё нет (UserID = uuid.Nil):
		// прежние операции такого пользователя не идентифицировать, гасить нечего
		if op.UserID != uuid.Nil {
			superseded = true

			err := o.storage.DeleteByUserIDAndName(ctx, op.UserID, op.Name)
			if err != nil {
				if !errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
					return err
				}

				// вытеснять было нечего - это штатный случай первой операции такого типа
				superseded = false
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

	// факт вытеснения фиксируется в журнале как отзыв
	if superseded {
		o.logOperation.Log(
			ctx,
			actor.NewOperationLog(
				op.Name, confirmmethod.Unspecified, logstatus.Revoked, logreason.Superseded,
			),
		)
	}

	// операция создана: фиксируем её инициацию в журнале
	o.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			op.Name, op.FirstActionMethod(), logstatus.Opened, logreason.Unspecified,
		),
	)

	return nil
}
