package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// SecureOperationPostgres - реализация хранилища защищённых операций в PostgreSQL.
	SecureOperationPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewSecureOperationPostgres - создаёт объект SecureOperationPostgres.
func NewSecureOperationPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *SecureOperationPostgres {
	return &SecureOperationPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// FetchOne - возвращает защищённую операцию по её токену.
func (re *SecureOperationPostgres) FetchOne(ctx context.Context, token string) (secureoperation.SecureOperation, error) {
	return re.fetchOne(ctx, token, false)
}

// FetchOneForUpdate - возвращает защищённую операцию по её токену, блокируя её строку
// (SELECT ... FOR UPDATE). Должен вызываться только внутри транзакции.
func (re *SecureOperationPostgres) FetchOneForUpdate(ctx context.Context, token string) (secureoperation.SecureOperation, error) {
	return re.fetchOne(ctx, token, true)
}

func (re *SecureOperationPostgres) fetchOne(ctx context.Context, token string, forUpdate bool) (row secureoperation.SecureOperation, err error) {
	sql := `
		SELECT
			operation_token,
			operation_name,
			user_id,
			confirm_actions,
			remaining_attempts,
			remaining_resends,
			resends_at,
			operation_payload,
			operation_status,
			expires_at
		FROM
			` + re.tableName + `
		WHERE
			operation_token = $1
		LIMIT 1`

	if forUpdate {
		sql += ` FOR UPDATE`
	}

	sql += `;`

	var (
		userID  *uuid.UUID
		actions []secureoperation.ConfirmAction
	)

	err = re.client.Conn(ctx).QueryRow(ctx, sql, token).Scan(
		&row.Token,
		&row.Name,
		&userID,
		&actions,
		&row.RemainingAttempts,
		&row.RemainingResends,
		&row.ResendsAt,
		&row.Payload,
		&row.Status,
		&row.ExpiresAt,
	)
	if err != nil {
		return secureoperation.SecureOperation{}, re.errorWrapper.Wrap(err)
	}

	// from nullable user_id field
	if userID != nil {
		row.UserID = *userID
	}

	// системное время: домен всегда оперирует UTC независимо от зоны сессии БД
	row.ResendsAt = row.ResendsAt.UTC()
	row.ExpiresAt = row.ExpiresAt.UTC()

	if err = secureoperation.WakeUp(&row, actions); err != nil {
		return secureoperation.SecureOperation{}, re.errorWrapper.Wrap(err)
	}

	return row, nil
}

// FetchByUserIDAndNames - возвращает действующие (не истёкшие) операции указанных типов
// указанного пользователя в любом статусе, упорядоченные по сроку действия. Если операций нет,
// возвращает пустой срез.
func (re *SecureOperationPostgres) FetchByUserIDAndNames(
	ctx context.Context,
	userID uuid.UUID,
	names []string,
) (rows []secureoperation.SecureOperation, err error) {
	sql := `
		SELECT
			operation_token,
			operation_name,
			confirm_actions,
			remaining_attempts,
			remaining_resends,
			resends_at,
			operation_payload,
			operation_status,
			expires_at
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1 AND operation_name = ANY($2) AND expires_at > NOW()
		ORDER BY
			expires_at, operation_token;`

	cursor, err := re.client.Conn(ctx).Query(ctx, sql, userID, names)
	if err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	defer cursor.Close()

	rows = make([]secureoperation.SecureOperation, 0)

	for cursor.Next() {
		var (
			row     = secureoperation.SecureOperation{UserID: userID}
			actions []secureoperation.ConfirmAction
		)

		err = cursor.Scan(
			&row.Token,
			&row.Name,
			&actions,
			&row.RemainingAttempts,
			&row.RemainingResends,
			&row.ResendsAt,
			&row.Payload,
			&row.Status,
			&row.ExpiresAt,
		)
		if err != nil {
			return nil, re.errorWrapper.Wrap(err)
		}

		// системное время: домен всегда оперирует UTC независимо от зоны сессии БД
		row.ResendsAt = row.ResendsAt.UTC()
		row.ExpiresAt = row.ExpiresAt.UTC()

		if err = secureoperation.WakeUp(&row, actions); err != nil {
			// операция истекла между отбором и разбором строки - её уже нет среди действующих
			if errors.Is(err, mrauth.ErrOperationAlreadyExpired) {
				continue
			}

			return nil, re.errorWrapper.Wrap(err)
		}

		rows = append(rows, row)
	}

	if err = cursor.Err(); err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	return rows, nil
}

// Insert - добавляет новую защищённую операцию.
func (re *SecureOperationPostgres) Insert(ctx context.Context, row secureoperation.SecureOperation) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				operation_token,
				operation_name,
				user_id,
				confirm_actions,
				remaining_attempts,
				remaining_resends,
				resends_at,
				operation_payload,
				operation_status,
				expires_at
			)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);`

	var userID *uuid.UUID

	// to nullable user_id field
	if row.UserID != uuid.Nil {
		userID = &row.UserID
	}

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.Token,
		row.Name,
		userID,
		row.Actions(),
		row.RemainingAttempts,
		row.RemainingResends,
		row.ResendsAt,
		row.Payload,
		row.Status,
		row.ExpiresAt,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// Replace - заменяет данные открытой операции (в статусе Opened).
func (re *SecureOperationPostgres) Replace(ctx context.Context, currentToken string, row secureoperation.SecureOperation) error {
	sql := `
        UPDATE
            ` + re.tableName + `
        SET
			operation_token = $3,
			confirm_actions = $4,
			remaining_attempts = $5,
			remaining_resends = $6,
			resends_at = $7,
			operation_status = $8,
			expires_at = $9
        WHERE
            operation_token = $1 AND operation_status = $2;`

	err := re.client.Conn(ctx).ExecRow(
		ctx,
		sql,
		currentToken,
		operationstatus.Opened,
		row.Token,
		row.Actions(),
		row.RemainingAttempts,
		row.RemainingResends,
		row.ResendsAt,
		row.Status,
		row.ExpiresAt,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// UpdateFailedAttempt - уменьшает счётчик оставшихся попыток подтверждения и возвращает его новое значение.
func (re *SecureOperationPostgres) UpdateFailedAttempt(ctx context.Context, token string) (attempts int16, err error) {
	sql := `
        UPDATE
            ` + re.tableName + `
        SET
			remaining_attempts = remaining_attempts - 1
        WHERE
            operation_token = $1 AND operation_status = $2
		RETURNING
			remaining_attempts;`

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		token,
		operationstatus.Opened,
	).Scan(
		&attempts,
	)
	if err != nil {
		return 0, re.errorWrapper.Wrap(err)
	}

	return attempts, nil
}

// DeleteByUserID - удаляет все операции указанного пользователя в любом статусе и возвращает
// их типы (по одному на удалённую операцию, возможны повторы). Если удалять было нечего,
// возвращает пустой срез без ошибки.
func (re *SecureOperationPostgres) DeleteByUserID(ctx context.Context, userID uuid.UUID) (names []string, err error) {
	sql := `
        DELETE FROM
            ` + re.tableName + `
        WHERE
            user_id = $1
        RETURNING
            operation_name;`

	cursor, err := re.client.Conn(ctx).Query(ctx, sql, userID)
	if err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	defer cursor.Close()

	names = make([]string, 0)

	for cursor.Next() {
		var name string

		if err = cursor.Scan(&name); err != nil {
			return nil, re.errorWrapper.Wrap(err)
		}

		names = append(names, name)
	}

	if err = cursor.Err(); err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	return names, nil
}

// DeleteByUserIDAndNames - удаляет операции указанных типов указанного пользователя в любом
// статусе (вытеснение прежних операций при открытии новой) и возвращает их типы (по одному
// на удалённую операцию, возможны повторы). Если удалять было нечего, возвращает пустой срез
// без ошибки.
func (re *SecureOperationPostgres) DeleteByUserIDAndNames(
	ctx context.Context,
	userID uuid.UUID,
	names []string,
) (deletedNames []string, err error) {
	sql := `
        DELETE FROM
            ` + re.tableName + `
        WHERE
            user_id = $1 AND operation_name = ANY($2)
        RETURNING
            operation_name;`

	cursor, err := re.client.Conn(ctx).Query(ctx, sql, userID, names)
	if err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	defer cursor.Close()

	deletedNames = make([]string, 0)

	for cursor.Next() {
		var name string

		if err = cursor.Scan(&name); err != nil {
			return nil, re.errorWrapper.Wrap(err)
		}

		deletedNames = append(deletedNames, name)
	}

	if err = cursor.Err(); err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	return deletedNames, nil
}

// Delete - удаляет защищённую операцию по её токену. Если операции уже нет
// (её потребил конкурентный запрос), возвращает errors.ErrEventStorageNoRecordFound.
func (re *SecureOperationPostgres) Delete(ctx context.Context, token string) error {
	sql := `
        DELETE FROM
            ` + re.tableName + `
        WHERE
            operation_token = $1;`

	err := re.client.Conn(ctx).ExecRow(
		ctx,
		sql,
		token,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// DeleteExpired - удаляет просроченные операции пакетом ограниченного размера (не более limit)
// и возвращает число фактически удалённых строк (сигнал "пачка была полной, есть ещё").
// Рассчитано на single-pod-планировщик (см. wire/mrauth/scheduler.NewService): конкурентной защиты на выборке нет.
func (re *SecureOperationPostgres) DeleteExpired(ctx context.Context, limit int) (count int, err error) {
	sql := `
		DELETE FROM
			` + re.tableName + ` t1
		USING (
			SELECT
				operation_token
			FROM
				` + re.tableName + `
			WHERE
				expires_at < NOW()
			ORDER BY
				expires_at ASC
			` + mrstorage.NonZeroLimit(limit) + `
		) ei
		WHERE
			t1.operation_token = ei.operation_token;`

	count, err = re.client.Conn(ctx).ExecAffected(ctx, sql)
	if err != nil {
		return 0, re.errorWrapper.Wrap(err)
	}

	return count, nil
}
