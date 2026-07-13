package collector

import (
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrprocess/collect"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtrace"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
)

const (
	defaultCaptionPrefix  = "OperationLog"
	defaultReadyTimeout   = 60 * time.Second
	defaultFlushPeriod    = 30 * time.Second
	defaultHandlerTimeout = 30 * time.Second
	defaultBatchSize      = 25
	defaultWorkersCount   = 1
)

// NewService - создаёт сервис для накопления и сохранения записей журнала защищённых операций.
func NewService(
	client mrstorage.DBConnManager,
	errorHandler errors.Handler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	secureOperationLogTableName string,
	opts ...Option,
) *collect.MessageCollector[entity.SecureOperationLog] {
	o := options{
		collectorOpts: []collect.Option[entity.SecureOperationLog]{
			collect.WithCaptionPrefix[entity.SecureOperationLog](defaultCaptionPrefix),
			collect.WithReadyTimeout[entity.SecureOperationLog](defaultReadyTimeout),
			collect.WithFlushPeriod[entity.SecureOperationLog](defaultFlushPeriod),
			collect.WithHandlerTimeout[entity.SecureOperationLog](defaultHandlerTimeout),
			collect.WithBatchSize[entity.SecureOperationLog](defaultBatchSize),
			collect.WithWorkersCount[entity.SecureOperationLog](defaultWorkersCount),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	statistic := operation.NewStatistic(
		repository.NewSecureOperationLogPostgres(
			client,
			secureOperationLogTableName,
		),
	)

	return collect.NewMessageCollector[entity.SecureOperationLog](
		statistic,
		errorHandler,
		logger,
		traceManager,
		o.collectorOpts...,
	)
}
