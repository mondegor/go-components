package repository

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// SecureOperationLogPostgres - репозиторий журнала защищённых операций.
	SecureOperationLogPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewSecureOperationLogPostgres - создаёт объект SecureOperationLogPostgres.
func NewSecureOperationLogPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *SecureOperationLogPostgres {
	return &SecureOperationLogPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// Insert - фиксирует пачку записей журнала защищённых операций.
func (re *SecureOperationLogPostgres) Insert(ctx context.Context, rows []entity.SecureOperationLog) error {
	if len(rows) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(rows))
	operations := make([]string, 0, len(rows))
	methods := make([]int16, 0, len(rows))
	statuses := make([]int16, 0, len(rows))
	reasons := make([]int16, 0, len(rows))
	clientIPs := make([]netip.Addr, 0, len(rows))
	clientProxyIPs := make([]netip.Addr, 0, len(rows))
	createdAts := make([]time.Time, 0, len(rows))

	for _, row := range rows {
		// защита от записи, собранной литералом (без конструктора): нулевое время события
		// записалось бы как 0001-01-01 и было бы снесено первым же проходом прунинга
		createdAt := row.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}

		ids = append(ids, row.VisitorID)
		operations = append(operations, row.OperationName)
		methods = append(methods, int16(row.ConfirmMethod))
		statuses = append(statuses, int16(row.LogStatus))
		reasons = append(reasons, int16(row.Reason))
		clientIPs = append(clientIPs, row.ClientIP.Real)
		clientProxyIPs = append(clientProxyIPs, row.ClientIP.Proxy)
		createdAts = append(createdAts, createdAt)
	}

	sql := `
		INSERT INTO ` + re.tableName + `
			(
				visitor_id,
				operation_name,
				confirm_method,
				log_status,
				reason,
				client_ip,
				client_proxy_ip,
				created_at
			)
		SELECT *
		FROM
			UNNEST($1::uuid[], $2::text[], $3::int2[], $4::int2[], $5::int2[], $6::inet[], $7::inet[], $8::timestamptz[])
			as t(visitor_id, operation_name, confirm_method, log_status, reason, client_ip, client_proxy_ip, created_at);`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		ids,
		operations,
		methods,
		statuses,
		reasons,
		clientIPs,
		clientProxyIPs,
		createdAts,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// DeleteBeforeDate - удаляет пачку записей лога операций старше datetime (не более limit)
// и возвращает число фактически удалённых строк (сигнал "пачка была полной, есть ещё").
// Рассчитано на single-pod-планировщик (см. wire/mrauth/scheduler.NewService): конкурентной защиты на выборке нет.
func (re *SecureOperationLogPostgres) DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) (count int, err error) {
	sql := `
		DELETE FROM
			` + re.tableName + ` t1
		USING (
			SELECT
				record_id
			FROM
				` + re.tableName + `
			WHERE
				created_at < $1
			ORDER BY
				created_at ASC
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
