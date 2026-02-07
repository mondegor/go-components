package operation

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// Statistic - comment struct.
	Statistic struct {
		storageLog   operationLogStorage
		errorWrapper errors.Wrapper
	}

	operationLogStorage interface {
		Insert(ctx context.Context, rows []entity.SecureOperationLog) error
	}
)

// NewStatistic - создаёт объект Session.
func NewStatistic(
	storageLog operationLogStorage,
) *Statistic {
	return &Statistic{
		storageLog:   storageLog,
		errorWrapper: errors.NewUseCaseWrapper(),
	}
}

// Execute - comments method.
func (uc *Statistic) Execute(ctx context.Context, list []entity.SecureOperationLog) error {
	if err := uc.storageLog.Insert(ctx, list); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
