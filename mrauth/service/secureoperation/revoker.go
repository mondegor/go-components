package secureoperation

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
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
		DeleteByUserID(ctx context.Context, userID uuid.UUID) (names []string, err error)
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

// RevokeAll - удаляет все незавершённые операции пользователя actor.VisitorID и фиксирует
// в журнале отзыв с причиной reason - по одной записи на каждый отозванный тип операции.
// Отсутствие операций - штатный случай, а не ошибка.
// Вызывается внутри транзакции изменения, из-за которого операции стали недействительными;
// журнал best-effort и пишется сразу, поэтому запись может пережить откат этой транзакции.
func (o *Revoker) RevokeAll(ctx context.Context, actor dto.ActorMeta, reason logreason.Enum) error {
	if actor.VisitorID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	names, err := o.storage.DeleteByUserID(ctx, actor.VisitorID)
	if err != nil {
		return o.errorWrapper.Wrap(err)
	}

	logged := make(map[string]struct{}, len(names))

	for _, name := range names {
		if _, ok := logged[name]; ok {
			continue
		}

		logged[name] = struct{}{}

		o.logOperation.Log(
			ctx,
			actor.NewOperationLog(name, confirmmethod.Unspecified, logstatus.Revoked, reason),
		)
	}

	return nil
}
