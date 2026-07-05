package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// UserActivityLogPostgres - репозиторий для хранения элементов настроек.
	UserActivityLogPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewUserActivityLogPostgres - создаёт объект UserActivityLogPostgres.
func NewUserActivityLogPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *UserActivityLogPostgres {
	return &UserActivityLogPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// Insert - фиксирует изменение настройки.
func (re *UserActivityLogPostgres) Insert(ctx context.Context, rows []dto.UserActivityLogMessage) error {
	if len(rows) == 0 {
		return nil
	}

	userIDs := make([]uuid.UUID, 0, len(rows))
	userIPs := make([]uint32, 0, len(rows))
	userIPSs := make([]string, 0, len(rows))
	userAgents := make([]string, 0, len(rows))
	requestPaths := make([]string, 0, len(rows))
	requestStatuses := make([]uint32, 0, len(rows))
	visitedAts := make([]time.Time, 0, len(rows))

	for _, row := range rows {
		realIP, _, err := row.UserIP.ToUint()
		if err != nil {
			return err // TODO: можно логировать ошибку
		}

		userIDs = append(userIDs, row.UserID)
		userIPs = append(userIPs, realIP)
		userIPSs = append(userIPSs, row.UserIP.String())
		userAgents = append(userAgents, row.UserAgent)
		requestPaths = append(requestPaths, row.RequestPath)
		requestStatuses = append(requestStatuses, row.RequestStatus)
		visitedAts = append(visitedAts, row.VisitedAt)
	}

	sql := `
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				user_ip,
				user_ip_string,
				user_agent,
				request_path,
				request_status,
				visited_at
			)
		SELECT *
		FROM
			UNNEST($1::uuid[], $2::int8[], $3::text[], $4::text[], $5::text[], $6::int4[], $7::timestamptz[])
			as t(user_id, user_ip, user_ip_string, user_agent, request_path, request_status, visited_at);`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		userIDs,
		userIPs,
		userIPSs,
		userAgents,
		requestPaths,
		requestStatuses,
		visitedAts,
	)
}

// DeleteBeforeDate - удаляет пачку записей лога активности старше datetime (не более limit)
// и возвращает число фактически удалённых строк (сигнал "пачка была полной, есть ещё").
// Рассчитано на single-pod-планировщик (см. wire/mrauth/scheduler.NewService): конкурентной защиты на выборке нет.
func (re *UserActivityLogPostgres) DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) (count int, err error) {
	sql := `
		DELETE FROM
			` + re.tableName + ` t1
		USING (
			SELECT
				record_id
			FROM
				` + re.tableName + `
			WHERE
				visited_at < $1
			ORDER BY
				visited_at ASC
			` + mrstorage.NonZeroLimit(limit) + `
		) ei
		WHERE
			t1.record_id = ei.record_id;`

	count, err = re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		datetime,
	)
	if err != nil {
		return 0, re.errorWrapper.Wrap(err)
	}

	return count, nil
}
