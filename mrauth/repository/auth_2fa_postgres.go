package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// Auth2faPostgres - хранилище данных 2FA пользователей в PostgreSQL.
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

// FetchOne - возвращает данные 2FA пользователя по его идентификатору.
func (re *Auth2faPostgres) FetchOne(ctx context.Context, userID uuid.UUID) (row entity.Auth2fa, err error) {
	sql := `
		SELECT
			auth_2fa_type,
			auth_secret,
			recovery_codes
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1
		LIMIT 1;`

	err = re.client.Conn(ctx).QueryRow(ctx, sql, userID).Scan(
		&row.Type,
		&row.Secret,
		&row.RecoveryCodes,
	)
	if err != nil {
		return entity.Auth2fa{}, re.errorWrapper.Wrap(err)
	}

	return row, nil
}

// InsertOrUpdate - создаёт или обновляет данные 2FA пользователя.
func (re *Auth2faPostgres) InsertOrUpdate(ctx context.Context, row entity.Auth2fa) error {
	// created_at = NOW() - время привязки 2FA (обновляется при каждой перепривязке);
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				auth_2fa_type,
				auth_secret,
				recovery_codes
			)
		VALUES
			($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET
			auth_2fa_type = EXCLUDED.auth_2fa_type,
			auth_secret = EXCLUDED.auth_secret,
			recovery_codes = EXCLUDED.recovery_codes,
			created_at = NOW(),
			updated_at = NOW();`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.UserID,
		row.Type,
		row.Secret,
		row.RecoveryCodes,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// ConsumeRecoveryCode - атомарно удаляет один хеш аварийного кода из набора пользователя.
// Если такого хеша нет (код уже израсходован параллельной операцией),
// возвращает errors.ErrEventStorageNoRecordFound.
func (re *Auth2faPostgres) ConsumeRecoveryCode(ctx context.Context, userID uuid.UUID, hash string) error {
	sql := `
		UPDATE
			` + re.tableName + `
		SET
			recovery_codes = recovery_codes - $2,
			updated_at = NOW()
		WHERE
			user_id = $1 AND jsonb_exists(recovery_codes, $2);`

	if err := re.client.Conn(ctx).Exec(ctx, sql, userID, hash); err != nil {
		if errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
			return errors.ErrEventStorageNoRecordFound
		}

		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// Delete - удаляет данные 2FA пользователя.
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
