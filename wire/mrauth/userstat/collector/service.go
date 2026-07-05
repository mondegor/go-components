package collector

import (
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrprocess/collect"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtrace"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/auth"
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
	usersActivityLogTableName,
	usersActivityStatTableName,
	sessionsTableName string,
	opts ...Option,
) *collect.MessageCollector[dto.UserActivityLogMessage] {
	o := options{
		collectorOpts: []collect.Option[dto.UserActivityLogMessage]{
			collect.WithCaptionPrefix[dto.UserActivityLogMessage](defaultCaptionPrefix),
			collect.WithReadyTimeout[dto.UserActivityLogMessage](defaultReadyTimeout),
			collect.WithFlushPeriod[dto.UserActivityLogMessage](defaultFlushPeriod),
			collect.WithHandlerTimeout[dto.UserActivityLogMessage](defaultHandlerTimeout),
			collect.WithBatchSize[dto.UserActivityLogMessage](defaultBatchSize),
			collect.WithWorkersCount[dto.UserActivityLogMessage](defaultWorkersCount),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	userStatistic := auth.NewUserStatistic(
		repository.NewUserActivityStatPostgres(
			client,
			usersActivityStatTableName,
		),
		repository.NewUserActivityLogPostgres(
			client,
			usersActivityLogTableName,
		),
		repository.NewSessionPostgres(
			client,
			sessionsTableName,
		),
	)

	return collect.NewMessageCollector[dto.UserActivityLogMessage](
		userStatistic,
		errorHandler,
		logger,
		traceManager,
		o.collectorOpts...,
	)
}
