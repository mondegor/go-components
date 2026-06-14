package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// Auth2faPostgres - comment struct.
	Auth2faPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewAuth2faPostgres - создаёт объект Auth2faPostgres.
func NewAuth2faPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *Auth2faPostgres {
	return &Auth2faPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// FetchOne - возвращает список сообщений по их указанным ID.
func (re *Auth2faPostgres) FetchOne(ctx context.Context, userID uuid.UUID) (row entity.Auth2fa, err error) {
	sql := `
		SELECT
			auth_2fa_type,
			auth_secret,
			cancel_secret
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1
		LIMIT 1;`

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		userID,
	).Scan(
		&row.Type,
		&row.Secret,
		&row.CancelSecret,
	)
	if err != nil {
		return entity.Auth2fa{}, re.errorWrapper.Wrap(err)
	}

	return row, nil
}

// InsertOrUpdate - возвращает список сообщений по их указанным ID.
func (re *Auth2faPostgres) InsertOrUpdate(ctx context.Context, row entity.Auth2fa) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				auth_2fa_type,
				auth_secret,
				cancel_secret
			)
		VALUES
			($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET
			auth_2fa_type = EXCLUDED.auth_2fa_type,
			auth_secret = EXCLUDED.auth_secret,
			cancel_secret = EXCLUDED.cancel_secret;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.UserID,
		row.Type,
		row.Secret,
		row.CancelSecret,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// Delete - comments method.
func (re *Auth2faPostgres) Delete(ctx context.Context, userID uuid.UUID) error {
	sql := `
		DELETE FROM
			` + re.tableName + `
		WHERE
			user_id = $1;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		userID,
	)
	// если это внутренняя ошибка
	if err != nil && !errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
