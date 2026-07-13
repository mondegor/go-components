package repository

import (
	"context"
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
	clientIPs := make([]uint32, 0, len(rows))
	clientIPSs := make([]string, 0, len(rows))
	createdAts := make([]time.Time, 0, len(rows))

	for _, row := range rows {
		// IPv4 сохраняется числом, для остальных адресов (IPv6) остаётся только строковое
		// представление: одна такая запись не должна срывать вставку всей пачки
		// TODO: все IPv6-клиенты попадают в один бакет client_ip=0, поэтому индекс по client_ip
		// и rate-limit по IP для них не работают; решить типом inet/bytea(16) или индексом по client_ip_str.
		// TODO: ToUint() возвращает ошибку, если не-IPv4 является ЛЮБОЙ из адресов (real, proxy),
		// поэтому валидный real IPv4 теряется при IPv6 в proxy; надёжнее брать row.ClientIP.Real.To4().
		realIP, _, err := row.ClientIP.ToUint()
		if err != nil {
			realIP = 0
		}

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
		clientIPs = append(clientIPs, realIP)
		clientIPSs = append(clientIPSs, row.ClientIP.String())
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
				client_ip_str,
				created_at
			)
		SELECT *
		FROM
			UNNEST($1::uuid[], $2::text[], $3::int2[], $4::int2[], $5::int2[], $6::int8[], $7::text[], $8::timestamptz[])
			as t(visitor_id, operation_name, confirm_method, log_status, reason, client_ip, client_ip_str, created_at);`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		ids,
		operations,
		methods,
		statuses,
		reasons,
		clientIPs,
		clientIPSs,
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
