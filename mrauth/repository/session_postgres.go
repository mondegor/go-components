package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// SessionPostgres - хранилище пользовательских сессий в PostgreSQL.
	SessionPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewSessionPostgres - создаёт объект SessionPostgres.
func NewSessionPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *SessionPostgres {
	return &SessionPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// Insert - сохраняет строку сессии при её открытии.
func (re *SessionPostgres) Insert(ctx context.Context, row entity.Session) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				session_id,
				user_agent,
				last_ip,
				updated_at
			)
		VALUES
			($1, $2, $3, $4, NOW());`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.UserID,
		row.SessionID,
		row.UserAgent,
		row.LastIP,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// FetchListByUserID - возвращает все строки сессий пользователя.
func (re *SessionPostgres) FetchListByUserID(ctx context.Context, userID uuid.UUID) ([]entity.Session, error) {
	sql := `
		SELECT
			session_id,
			COALESCE(user_agent, ''),
			COALESCE(last_ip, 0)
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1;`

	cursor, err := re.client.Conn(ctx).Query(
		ctx,
		sql,
		userID,
	)
	if err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	defer cursor.Close()

	rows := make([]entity.Session, 0)

	for cursor.Next() {
		row := entity.Session{
			UserID: userID,
		}

		if err = cursor.Scan(
			&row.SessionID,
			&row.UserAgent,
			&row.LastIP,
		); err != nil {
			return nil, re.errorWrapper.Wrap(err)
		}

		rows = append(rows, row)
	}

	if err = cursor.Err(); err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	return rows, nil
}
