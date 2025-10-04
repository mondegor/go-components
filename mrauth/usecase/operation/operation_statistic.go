package operation

import (
	"context"

	"github.com/mondegor/go-sysmess/mrerr"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// Statistic - компонент для извлечения настроек, которые хранятся в хранилище данных.
	Statistic struct {
		storageLog   mrauth.SecureOperationLogStorage
		errorWrapper mrerr.UseCaseErrorWrapper
	}
)

// NewStatistic - создаёт объект Session.
func NewStatistic(
	storageLog mrauth.SecureOperationLogStorage,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *Statistic {
	return &Statistic{
		storageLog:   storageLog,
		errorWrapper: mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameSecureOperation),
	}
}

// Execute - comments method.
func (uc *Statistic) Execute(ctx context.Context, list []dto.SecureOperationLog) error {
	if err := uc.storageLog.Insert(ctx, list); err != nil {
		return uc.errorWrapper.WrapErrorFailed(err)
	}

	return nil
}
