package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserRealmPostgres - репозиторий для хранения сообщений подготовленных для отправки различным получателям.
	UserRealmPostgres struct {
		client       mrstorage.DBConnManager
		tableName    string
		errorWrapper core.ErrorWrapper
	}
)

// NewUserRealmPostgres - создаёт объект UserRealmPostgres.
func NewUserRealmPostgres(client mrstorage.DBConnManager, tableName string) *UserRealmPostgres {
	return &UserRealmPostgres{
		client:       client,
		tableName:    tableName,
		errorWrapper: core.NewStorageErrorWrapper(tableName),
	}
}

// Fetch - comments method.
func (re *UserRealmPostgres) Fetch(ctx context.Context, userID uuid.UUID) ([]entity.UserRealm, error) {
	sql := `
        SELECT
            user_realm,
			user_kind
        FROM
            ` + re.tableName + `
        WHERE
            user_id = $1
        ORDER BY
            user_realm ASC;`

	cursor, err := re.client.Conn(ctx).Query(
		ctx,
		sql,
		userID,
	)
	if err != nil {
		return nil, err
	}

	defer cursor.Close()

	rows := make([]entity.UserRealm, 0)

	for cursor.Next() {
		row := entity.UserRealm{ // ??????????????????????????????????????
			UserID: userID,
		}

		err = cursor.Scan(
			&row.Realm,
			&row.Kind,
		)
		if err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}

	return rows, cursor.Err()
}

// FetchOne - возвращает список сообщений по их указанным SettingID.
func (re *UserRealmPostgres) FetchOne(ctx context.Context, userID uuid.UUID, realm string) (row entity.UserRealm, err error) {
	sql := `
		SELECT
			user_kind
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1 AND user_realm = $2
		LIMIT 1;`

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		userID,
		realm,
	).Scan(
		&row.Kind,
	)
	if err != nil {
		return entity.UserRealm{}, re.errorWrapper.WrapError(err)
	}

	row.UserID = userID
	row.Realm = realm

	return row, nil
}

// Insert - возвращает список сообщений по их указанным SettingID.
func (re *UserRealmPostgres) Insert(ctx context.Context, row entity.UserRealm) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				user_realm,
				user_kind
			)
		VALUES
			($1, $2, $3);`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.UserID,
		row.Realm,
		row.Kind,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err)
	}

	return nil
}

// UpdateKind - возвращает список сообщений по их указанным SettingID.
func (re *UserRealmPostgres) UpdateKind(ctx context.Context, row entity.UserRealm) error {
	sql := `
        UPDATE
            ` + re.tableName + `
        SET
			user_kind = $3,
			updated_at = NOW()
        WHERE
            user_id = $1 AND user_realm = $2;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.UserID,
		row.Realm,
		row.Kind,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err)
	}

	return nil
}

// Delete - comments method.
func (re *UserRealmPostgres) Delete(ctx context.Context, userID uuid.UUID, realm string) error {
	sql := `
		DELETE FROM
			` + re.tableName + `
		WHERE
			user_id = $1 AND user_realm = $2;`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		userID,
		realm,
	)
}
