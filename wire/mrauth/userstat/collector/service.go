package collector

import (
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrtrace"
	"github.com/mondegor/go-webcore/mrworker/process/collect"

	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/auth"
	"github.com/mondegor/go-components/mrauth/usecase/auth/handle"
)

const (
	defaultCaptionPrefix  = "UserStatRequest"
	defaultReadyTimeout   = 60 * time.Second
	defaultFlushPeriod    = 30 * time.Second
	defaultHandlerTimeout = 30 * time.Second
	defaultBatchSize      = 25
	defaultWorkersCount   = 1
)

// NewService - создаёт сервис для обработки и отправки сообщений и связанных с ним задачи.
func NewService(
	client mrstorage.DBConnManager,
	errorHandler errors.Handler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	userActivityStatTable mrsql.DBTableInfo,
	userActivityLogTable string,
	opts ...Option,
) *collect.MessageCollector {
	o := options{
		collectorOpts: []collect.Option{
			collect.WithCaptionPrefix(defaultCaptionPrefix),
			collect.WithReadyTimeout(defaultReadyTimeout),
			collect.WithFlushPeriod(defaultFlushPeriod),
			collect.WithHandlerTimeout(defaultHandlerTimeout),
			collect.WithBatchSize(defaultBatchSize),
			collect.WithWorkersCount(defaultWorkersCount),
		},
		handlerOpts: nil,
	}

	for _, opt := range opts {
		opt(&o)
	}

	userStatistic := auth.NewUserStatistic(
		repository.NewUserActivityStatPostgres(
			client,
			userActivityStatTable,
		),
		repository.NewUserActivityLogPostgres(
			client,
			userActivityLogTable,
		),
	)

	return collect.NewMessageCollector(
		handle.New(
			userStatistic,
			o.handlerOpts...,
		),
		errorHandler,
		logger,
		traceManager,
		o.collectorOpts...,
	)
}
