package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/authtokenstatus"
)

type (
	// AuthTokenPostgres - comment struct.
	AuthTokenPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		table        mrsql.DBTableInfo
	}
)

var (
	// ErrTokenExpired - token is expired.
	ErrTokenExpired = errors.New("token is expired")

	// ErrTokenAlreadyRevoked - token is already revoked.
	ErrTokenAlreadyRevoked = errors.New("token is already revoked")
)

// NewAuthTokenPostgres - создаёт объект AuthTokenPostgres.
func NewAuthTokenPostgres(
	client mrstorage.DBConnManager,
	table mrsql.DBTableInfo,
) *AuthTokenPostgres {
	return &AuthTokenPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		table:        table,
	}
}

// FetchOne - возвращает список сообщений по их указанным ID.
func (re *AuthTokenPostgres) FetchOne(ctx context.Context, accessToken string) (row dto.AuthTokenScopes, err error) {
	sql := `
		SELECT
			user_id,
			token_scopes,
			(access_expires_at <= NOW()) as is_expired
		FROM
			` + re.table.Name + `
		WHERE
			access_token = $1 AND token_status = $2
		LIMIT 1;`

	var (
		userID    uuid.UUID
		isExpired bool
	)

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		accessToken,
		authtokenstatus.Opened,
	).Scan(
		&userID,
		&row, // загружаются все данные кроме userID, т.к. хранится в отдельном поле
		&isExpired,
	)
	if err != nil {
		return dto.AuthTokenScopes{}, re.errorWrapper.Wrap(err)
	}

	if isExpired {
		return dto.AuthTokenScopes{}, ErrTokenExpired
	}

	// дозаполняется структура userID полученного из отдельного поля
	row.UserID = userID

	return row, nil
}

// Insert - возвращает список сообщений по их указанным ID.
func (re *AuthTokenPostgres) Insert(ctx context.Context, row entity.AuthToken) error {
	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				access_token,
				access_expires_at,
				user_id,
				token_scopes,
				token_status,
				expires_at
			)
		VALUES
			($1, $2, $3, $4, $5, $6, $7);`

	var accessToken *string

	if row.AccessToken != "" {
		accessToken = &row.AccessToken
	}

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.RefreshToken,
		accessToken,
		row.AccessExpiresAt,
		row.Scopes.UserID,
		row.Scopes, // userID будет исключён, т.к. он сохраняется в отдельном поле
		authtokenstatus.Opened,
		row.ExpiresAt,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// UpdateToClose - comments method.
func (re *AuthTokenPostgres) UpdateToClose(ctx context.Context, accessToken string) error {
	sql := `
        UPDATE
            ` + re.table.Name + `
        SET
            token_status = $3
        WHERE
            access_token = $1 AND token_status = $2 AND access_expires_at > NOW();`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		accessToken,
		authtokenstatus.Opened,
		authtokenstatus.Closed,
	)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageRowsNotAffected) {
			err = errors.ErrEventStorageNoRowFound
		}

		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// Revoke - comments method.
func (re *AuthTokenPostgres) Revoke(ctx context.Context, refreshToken string) (row dto.AuthTokenScopes, err error) {
	sql := `
        UPDATE
            ` + re.table.Name + `
        SET
			token_status = (CASE WHEN token_status = $2 THEN $3 ELSE $4 END)
        WHERE
            ` + re.table.PrimaryKey + ` = $1 AND token_status IN ($2, $3)
		RETURNING
			user_id,
			token_scopes,
			(expires_at <= NOW()) as is_expired,
			(token_status = $4) as already_revoked;`

	var (
		userID           uuid.UUID
		isExpired        bool
		isAlreadyRevoked bool
	)

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		refreshToken,
		authtokenstatus.Opened,
		authtokenstatus.Revoked,
		authtokenstatus.UnexpectedRevoked,
	).Scan(
		&userID,
		&row, // загружаются все данные кроме userID, т.к. это поле хранится отдельно
		&isExpired,
		&isAlreadyRevoked,
	)
	if err != nil {
		return dto.AuthTokenScopes{}, re.errorWrapper.Wrap(err)
	}

	if isExpired {
		return dto.AuthTokenScopes{}, ErrTokenExpired
	}

	// дозаполняется структура userID полученного из отдельного поля
	row.UserID = userID

	if isAlreadyRevoked {
		return row, ErrTokenAlreadyRevoked
	}

	return row, nil
}

// UpdateToCloseAll - comments method.
func (re *AuthTokenPostgres) UpdateToCloseAll(ctx context.Context, userID uuid.UUID) error {
	sql := `
        UPDATE
            ` + re.table.Name + `
        SET
			token_status = $3
        WHERE
            user_id = $1 AND token_status = $2;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		userID,
		authtokenstatus.Opened,
		authtokenstatus.Closed,
	)
	// если это внутренняя ошибка
	if err != nil && !errors.Is(err, errors.ErrEventStorageRowsNotAffected) {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// DeleteExpired - comments method.
func (re *AuthTokenPostgres) DeleteExpired(ctx context.Context, limit int) error {
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
	// если это внутренняя ошибка
	if err != nil && !errors.Is(err, errors.ErrEventStorageRowsNotAffected) {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
