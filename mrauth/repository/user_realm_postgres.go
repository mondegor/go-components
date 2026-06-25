package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserRealmPostgres - хранилище привязок пользователей к realm в PostgreSQL.
	UserRealmPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewUserRealmPostgres - создаёт объект UserRealmPostgres.
func NewUserRealmPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *UserRealmPostgres {
	return &UserRealmPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// Fetch - возвращает список realm пользователя с их видами.
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

// FetchOne - возвращает вид пользователя в указанном realm.
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
		return entity.UserRealm{}, re.errorWrapper.Wrap(err)
	}

	row.UserID = userID
	row.Realm = realm

	return row, nil
}

// Insert - добавляет привязку пользователя к realm.
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
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// UpdateKind - обновляет вид пользователя в указанном realm.
func (re *UserRealmPostgres) UpdateKind(ctx context.Context, row entity.UserRealm) error {
	sql := `
        UPDATE
            ` + re.tableName + `
        SET
			user_kind = $3,
			updated_at = NOW()
        WHERE
            user_id = $1 AND user_realm = $2;`

	err := re.client.Conn(ctx).ExecRow(
		ctx,
		sql,
		row.UserID,
		row.Realm,
		row.Kind,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// Delete - удаляет привязку пользователя к указанному realm.
func (re *UserRealmPostgres) Delete(ctx context.Context, userID uuid.UUID, realm string) error {
	sql := `
		DELETE FROM
			` + re.tableName + `
		WHERE
			user_id = $1 AND user_realm = $2;`

	err := re.client.Conn(ctx).ExecRow(
		ctx,
		sql,
		userID,
		realm,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
