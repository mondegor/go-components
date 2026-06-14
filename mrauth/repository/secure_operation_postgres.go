package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// SecureOperationPostgres - comment struct.
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

// FetchOne - возвращает список сообщений по их указанным ID.
func (re *SecureOperationPostgres) FetchOne(ctx context.Context, token string) (row secureoperation.SecureOperation, err error) {
	sql := `
		SELECT
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
		LIMIT 1;`

	var (
		userID  *uuid.UUID
		actions []secureoperation.ConfirmAction
	)

	row.Token = token

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		token,
	).Scan(
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

	if err = secureoperation.WakeUp(&row, actions); err != nil {
		return secureoperation.SecureOperation{}, re.errorWrapper.Wrap(err)
	}

	return row, nil
}

// Insert - возвращает список сообщений по их указанным ID.
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

// Replace - comments method.
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

	err := re.client.Conn(ctx).Exec(
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
		if errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
			return errors.ErrEventStorageNoRecordFound
		}

		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// UpdateFailedAttempt - comments method.
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

// Delete - comments method.
func (re *SecureOperationPostgres) Delete(ctx context.Context, token string) error {
	sql := `
        DELETE FROM
            ` + re.tableName + `
        WHERE
            operation_token = $1;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		token,
	)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
			return errors.ErrEventStorageNoRecordFound
		}

		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// DeleteExpired - comments method.
func (re *SecureOperationPostgres) DeleteExpired(ctx context.Context, limit int) error {
	sql := `
		WITH expired_items as (
			SELECT
			  	operation_token as item_id
			FROM
			  	` + re.tableName + `
			WHERE
				expires_at < NOW()
			ORDER BY
				expires_at ASC
		    LIMIT $1
		)
		DELETE FROM
			` + re.tableName + ` t1
		USING
			expired_items ei
		WHERE
			t1.operation_token = ei.item_id;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		limit,
	)
	// если это внутренняя ошибка
	if err != nil && !errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
