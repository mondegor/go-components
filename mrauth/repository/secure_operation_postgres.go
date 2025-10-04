package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
)

type (
	// SecureOperationPostgres - репозиторий для хранения сообщений подготовленных для отправки различным получателям.
	SecureOperationPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper mrerr.ErrorWrapper
		table        mrsql.DBTableInfo
	}
)

// NewSecureOperationPostgres - создаёт объект SecureOperationPostgres.
func NewSecureOperationPostgres(
	client mrstorage.DBConnManager,
	errorWrapper mrerr.ErrorWrapper,
	table mrsql.DBTableInfo,
) *SecureOperationPostgres {
	return &SecureOperationPostgres{
		client:       client,
		errorWrapper: mrerr.NewErrorWrapper(errorWrapper, table.Name),
		table:        table,
	}
}

// FetchOne - возвращает список сообщений по их указанным SettingID.
func (re *SecureOperationPostgres) FetchOne(ctx context.Context, token string) (row entity.SecureOperation, err error) {
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
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = $1
		LIMIT 1;`

	var userID *uuid.UUID

	row.Token = token

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		token,
	).Scan(
		&row.Name,
		&userID,
		&row.Actions,
		&row.RemainingAttempts,
		&row.RemainingResends,
		&row.ResendsAt,
		&row.Payload,
		&row.Status,
		&row.ExpiresAt,
	)
	if err != nil {
		return entity.SecureOperation{}, re.errorWrapper.WrapError(err)
	}

	// from nullable user_id field
	if userID != nil {
		row.UserID = *userID
	}

	return row, nil
}

// Insert - возвращает список сообщений по их указанным SettingID.
func (re *SecureOperationPostgres) Insert(ctx context.Context, row entity.SecureOperation) error {
	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
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
		row.Actions,
		row.RemainingAttempts,
		row.RemainingResends,
		row.ResendsAt,
		row.Payload,
		row.Status,
		row.ExpiresAt,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err)
	}

	return nil
}

// Update - comments method.
func (re *SecureOperationPostgres) Update(ctx context.Context, currentToken string, row entity.SecureOperation) error {
	sql := `
        UPDATE
            ` + re.table.Name + `
        SET
			operation_token = $3,
			confirm_actions = $4,
			remaining_attempts = $5,
			remaining_resends = $6,
			resends_at = $7,
			operation_status = $8,
			expires_at = $9
        WHERE
            ` + re.table.PrimaryKey + ` = $1 AND operation_status = $2;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		currentToken,
		enum.OperationStatusOpened,
		row.Token,
		row.Actions,
		row.RemainingAttempts,
		row.RemainingResends,
		row.ResendsAt,
		row.Status,
		row.ExpiresAt,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err)
	}

	return nil
}

// UpdateFailedAttempt - comments method.
func (re *SecureOperationPostgres) UpdateFailedAttempt(ctx context.Context, token string) (attempts uint32, err error) {
	sql := `
        UPDATE
            ` + re.table.Name + `
        SET
			remaining_attempts = remaining_attempts - 1
        WHERE
            ` + re.table.PrimaryKey + ` = $1 AND operation_status = $2
		RETURNING
			remaining_attempts;`

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		token,
		enum.OperationStatusOpened,
	).Scan(
		&attempts,
	)
	if err != nil {
		return 0, re.errorWrapper.WrapError(err)
	}

	return attempts, nil
}

// Delete - comments method.
func (re *SecureOperationPostgres) Delete(ctx context.Context, token string) error {
	sql := `
        DELETE FROM
            ` + re.table.Name + `
        WHERE
            ` + re.table.PrimaryKey + ` = $1;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		token,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err)
	}

	return nil
}

// DeleteExpired - comments method.
func (re *SecureOperationPostgres) DeleteExpired(ctx context.Context, limit int) error {
	sql := `
		WITH expired_items as (
			SELECT
			  	` + re.table.PrimaryKey + ` as item_id
			FROM
			  	` + re.table.Name + `
			WHERE
				expires_at < NOW()
			ORDER BY
				expires_at ASC
		    LIMIT $1
		)
		DELETE FROM
			` + re.table.Name + ` t1
		USING
			expired_items ei
		WHERE
			t1.` + re.table.PrimaryKey + ` = ei.item_id;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		limit,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err)
	}

	return nil
}
