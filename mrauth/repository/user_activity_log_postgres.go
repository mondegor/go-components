package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"

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

// DeleteBeforeDate - comments method.
func (re *UserActivityLogPostgres) DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) error {
	sql := `
		WITH old_items as (
			SELECT
			  	record_id as item_id
			FROM
			  	` + re.tableName + `
			WHERE
				visited_at < $1
			ORDER BY
				visited_at ASC
		    LIMIT $2
		)
		DELETE FROM
			` + re.tableName + ` t1
		USING
			old_items ei
		WHERE
			t1.record_id = ei.item_id;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		datetime,
		limit,
	)
	// если это внутренняя ошибка
	if err != nil && !errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
