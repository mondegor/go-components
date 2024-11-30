package change

import (
	"context"

	"github.com/mondegor/go-components/mrqueue"
)

type (
	// StatusChanger - объект изменяющий статусы сломавшихся сообщений, находящихся в очереди.
	StatusChanger struct {
		queueChanger mrqueue.Changer
	}
)

// New - создаёт объект StatusChanger.
func New(
	queueChanger mrqueue.Changer,
) *StatusChanger {
	return &StatusChanger{
		queueChanger: queueChanger,
	}
}

// ChangeProcessingToRetryByTimeout - переводит ограниченный список сообщений из статуса PROCESSING
// в статус RETRY по таймауту (например, в случае если обработка сообщений подвисла) с занесением события в журнал ошибок.
func (co *StatusChanger) ChangeProcessingToRetryByTimeout(ctx context.Context, limit uint32) error {
	_, err := co.queueChanger.ChangeProcessingToRetryByTimeout(ctx, limit)

	return err
}

// ChangeRetryToReady - переводит ограниченный список сообщений из статуса RETRY в статус READY
// учитывая указанную задержку нахождения сообщения в этом статусе и положительное кол-во попыток.
func (co *StatusChanger) ChangeRetryToReady(ctx context.Context, limit uint32) error {
	_, err := co.queueChanger.ChangeRetryToReady(ctx, limit)

	return err
}