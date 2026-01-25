package operation

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// Statistic - comment struct.
	Statistic struct {
		storageLog   mrauth.SecureOperationLogStorage
		errorWrapper errors.Wrapper
	}
)

// NewStatistic - создаёт объект Session.
func NewStatistic(
	storageLog mrauth.SecureOperationLogStorage,
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
