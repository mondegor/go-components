package secureoperation

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
)

type (
	// Revoker - отзыв всех незавершённых защищённых операций пользователя, включая подтверждённые,
	// но ещё не применённые. Применяется, когда
	// изменилось то, по чему строились цепочки подтверждения его операций (например, состояние
	// 2FA): операции, созданные при прежнем состоянии, продолжать нельзя - их нужно начать заново.
	Revoker struct {
		storage      operationRevokerStorage
		logOperation operationLogger
		errorWrapper errors.Wrapper
	}

	operationRevokerStorage interface {
		DeleteByUserID(ctx context.Context, userID uuid.UUID) (types []operationtype.Enum, err error)
	}
)

// NewRevoker - создаёт объект Revoker.
func NewRevoker(storage operationRevokerStorage, logOperation operationLogger) *Revoker {
	return &Revoker{
		storage:      storage,
		logOperation: logOperation,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// RevokeAll - удаляет все незавершённые операции пользователя actor.UserID и фиксирует
// в журнале отзыв с причиной reason - по одной записи на каждый отозванный тип операции.
// Отсутствие операций - штатный случай, а не ошибка.
// Вызывается внутри транзакции изменения, из-за которого операции стали недействительными;
// журнал best-effort и в транзакции не участвует, поэтому запись может пережить её откат.
func (o *Revoker) RevokeAll(ctx context.Context, actor dto.ActorMeta, reason logreason.Enum) error {
	if actor.UserID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	types, err := o.storage.DeleteByUserID(ctx, actor.UserID)
	if err != nil {
		return o.errorWrapper.Wrap(err)
	}

	logged := make(map[operationtype.Enum]struct{}, len(types))

	for _, opType := range types {
		if _, ok := logged[opType]; ok {
			continue
		}

		logged[opType] = struct{}{}

		o.logOperation.Log(
			ctx,
			actor.NewOperationLog(opType.String(), confirmmethod.Unspecified, logstatus.Revoked, reason),
		)
	}

	return nil
}
