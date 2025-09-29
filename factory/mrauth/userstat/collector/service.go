package collector

import (
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-webcore/mrworker/process/collect"

	core "github.com/mondegor/go-components/internal"
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

type (
	serviceOptions struct {
		requestCollector []collect.Option
		requestHandler   []handle.Option
	}
)

// NewService - создаёт сервис для обработки и отправки сообщений и связанных с ним задачи.
func NewService(
	client mrstorage.DBConnManager,
	errorHandler core.ErrorHandler,
	logger mrlog.Logger,
	traceManager core.TraceManager,
	userActivityStatTable mrsql.DBTableInfo,
	userActivityLogTable string,
	opts ...ServiceOption,
) *collect.MessageCollector {
	o := serviceOptions{
		requestCollector: []collect.Option{
			collect.WithCaptionPrefix(defaultCaptionPrefix),
			collect.WithReadyTimeout(defaultReadyTimeout),
			collect.WithFlushPeriod(defaultFlushPeriod),
			collect.WithHandlerTimeout(defaultHandlerTimeout),
			collect.WithBatchSize(defaultBatchSize),
			collect.WithWorkersCount(defaultWorkersCount),
		},
		requestHandler: nil,
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
			o.requestHandler...,
		),
		errorHandler,
		logger,
		traceManager,
		o.requestCollector...,
	)
}
