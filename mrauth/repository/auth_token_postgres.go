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
	// AuthTokenPostgres - репозиторий для хранения сообщений подготовленных для отправки различным получателям.
	AuthTokenPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper mrerr.ErrorWrapper
		table        mrsql.DBTableInfo
	}
)

var (
	// ErrTokenExpired - token is expired.
	ErrTokenExpired = mrerr.NewKindInternal("token is expired", mrerr.WithDisabledCaller(), mrerr.WithDisabledOnCreated())

	// ErrTokenAlreadyRevoked - token is already revoked.
	ErrTokenAlreadyRevoked = mrerr.NewKindInternal("token is already revoked", mrerr.WithDisabledCaller(), mrerr.WithDisabledOnCreated())
)

// NewAuthTokenPostgres - создаёт объект AuthTokenPostgres.
func NewAuthTokenPostgres(
	client mrstorage.DBConnManager,
	errorWrapper mrerr.ErrorWrapper,
	table mrsql.DBTableInfo,
) *AuthTokenPostgres {
	return &AuthTokenPostgres{
		client:       client,
		errorWrapper: mrerr.NewErrorWrapper(errorWrapper, table.Name),
		table:        table,
	}
}

// FetchOne - возвращает список сообщений по их указанным SettingID.
func (re *AuthTokenPostgres) FetchOne(ctx context.Context, accessToken string) (row entity.AuthTokenScopes, err error) {
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
		enum.AuthTokenStatusOpened,
	).Scan(
		&userID,
		&row, // загружаются все данные кроме userID, т.к. хранится в отдельном поле
		&isExpired,
	)
	if err != nil {
		return entity.AuthTokenScopes{}, re.errorWrapper.WrapError(err)
	}

	if isExpired {
		return entity.AuthTokenScopes{}, ErrTokenExpired
	}

	// дозаполняется структура userID полученного из отдельного поля
	row.UserID = userID

	return row, nil
}

// Insert - возвращает список сообщений по их указанным SettingID.
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
		enum.AuthTokenStatusOpened,
		row.ExpiresAt,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err)
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
		enum.AuthTokenStatusOpened,
		enum.AuthTokenStatusClosed,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err)
	}

	return nil
}

// Revoke - comments method.
func (re *AuthTokenPostgres) Revoke(ctx context.Context, refreshToken string) (row entity.AuthTokenScopes, err error) {
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
		enum.AuthTokenStatusOpened,
		enum.AuthTokenStatusRevoked,
		enum.AuthTokenStatusUnexpectedRevoked,
	).Scan(
		&userID,
		&row, // загружаются все данные кроме userID, т.к. хранится в отдельном поле
		&isExpired,
		&isAlreadyRevoked,
	)
	if err != nil {
		return entity.AuthTokenScopes{}, re.errorWrapper.WrapError(err)
	}

	if isExpired {
		return entity.AuthTokenScopes{}, ErrTokenExpired
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
		enum.AuthTokenStatusOpened,
		enum.AuthTokenStatusClosed,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err)
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
	if err != nil {
		return re.errorWrapper.WrapError(err)
	}

	return nil
}
