package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// Auth2faPostgres - репозиторий для хранения сообщений подготовленных для отправки различным получателям.
	Auth2faPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper mrerr.ErrorWrapper
		table        mrsql.DBTableInfo
	}
)

// NewAuth2faPostgres - создаёт объект Auth2faPostgres.
func NewAuth2faPostgres(
	client mrstorage.DBConnManager,
	errorWrapper mrerr.ErrorWrapper,
	table mrsql.DBTableInfo,
) *Auth2faPostgres {
	return &Auth2faPostgres{
		client:       client,
		errorWrapper: mrerr.NewErrorWrapper(errorWrapper, table.Name),
		table:        table,
	}
}

// FetchOne - возвращает список сообщений по их указанным SettingID.
func (re *Auth2faPostgres) FetchOne(ctx context.Context, userID uuid.UUID) (row entity.Auth2fa, err error) {
	sql := `
		SELECT
			auth_2fa_type,
			auth_secret,
			cancel_secret
		FROM
			` + re.table.Name + `
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
		return entity.Auth2fa{}, re.errorWrapper.WrapError(err)
	}

	return row, nil
}

// InsertOrUpdate - возвращает список сообщений по их указанным SettingID.
func (re *Auth2faPostgres) InsertOrUpdate(ctx context.Context, row entity.Auth2fa) error {
	sql := `
		INSERT INTO ` + re.table.Name + `
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
		return re.errorWrapper.WrapError(err)
	}

	return nil
}

// Delete - comments method.
func (re *Auth2faPostgres) Delete(ctx context.Context, userID uuid.UUID) error {
	sql := `
		DELETE FROM
			` + re.table.Name + `
		WHERE
			user_id = $1;`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		userID,
	)
}
