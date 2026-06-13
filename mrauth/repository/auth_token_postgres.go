package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrstorage/mrsql"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/authtokenstatus"
	"github.com/mondegor/go-components/mrauth/enum/authtokentype"
)

type (
	// AuthTokenPostgres - хранилище токенов авторизации в PostgreSQL.
	AuthTokenPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		table        mrsql.DBTableInfo
	}
)

// ErrTokenExpired - token is expired.
var ErrTokenExpired = errors.New("token is expired")

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

// FetchOneByAccessToken - возвращает область действия пользователя по действующему access токену.
func (re *AuthTokenPostgres) FetchOneByAccessToken(ctx context.Context, accessToken string) (row dto.UserScopes, err error) {
	sql := `
		SELECT
			user_id,
			session_id,
			token_scopes,
			(expires_at <= NOW()) as is_expired
		FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = $1 AND token_type = $2 AND token_status = $3
		LIMIT 1;`

	var (
		userID    uuid.UUID
		sessionID uint32
		isExpired bool
	)

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		accessToken,
		authtokentype.Access,
		authtokenstatus.Enabled,
	).Scan(
		&userID,
		&sessionID,
		&row, // загружаются все данные кроме userID/sessionID, т.к. они хранятся в отдельных полях
		&isExpired,
	)
	if err != nil {
		return dto.UserScopes{}, re.errorWrapper.Wrap(err)
	}

	if isExpired {
		return dto.UserScopes{}, ErrTokenExpired
	}

	// дозаполняется структура данными, полученными из отдельных полей
	row.UserID = userID
	row.SessionID = sessionID

	return row, nil
}

// Insert - сохраняет несколько токенов авторизации.
func (re *AuthTokenPostgres) Insert(ctx context.Context, rows []entity.AuthToken) error {
	if len(rows) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(rows))
	tokenTypes := make([]int16, 0, len(rows))
	userIDs := make([]uuid.UUID, 0, len(rows))
	sessionIDs := make([]uint32, 0, len(rows))
	scopes := make([]entity.AuthTokenScopes, 0, len(rows))
	expiresAts := make([]time.Time, 0, len(rows))

	for _, row := range rows {
		tokens = append(tokens, row.Token)
		tokenTypes = append(tokenTypes, int16(row.Type))
		userIDs = append(userIDs, row.UserID)
		sessionIDs = append(sessionIDs, row.SessionID)
		scopes = append(scopes, row.Scopes) // userID/sessionID будут исключены, т.к. они сохраняются в отдельных полях
		expiresAts = append(expiresAts, row.ExpiresAt)
	}

	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				token_type,
				user_id,
				session_id,
				token_scopes,
				token_status,
				expires_at
			)
		SELECT token, token_type, user_id, session_id, token_scopes, $7, expires_at
		FROM
			UNNEST($1::varchar[], $2::int2[], $3::uuid[], $4::int8[], $5::jsonb[], $6::timestamptz[])
			as t(token, token_type, user_id, session_id, token_scopes, expires_at);`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		tokens,
		tokenTypes,
		userIDs,
		sessionIDs,
		scopes,
		expiresAts,
		authtokenstatus.Enabled,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// RevokeRefresh - переводит действующий refresh токен в статус "отозван" и устанавливает ему
// окно идемпотентности длиной grace, в течение которого допустим повторный (идемпотентный) запрос.
//
// Возможные исходы:
//   - isRetried=false - токен был действующим и отозван, требуется выпуск новой пары токенов;
//   - isRetried=true - токен уже отозван, но окно его действия ещё не закрыто,
//     требуется вернуть текущую (последнюю) пару токенов сессии;
//   - ErrTokenExpired - токен истёк (не был отозван);
//   - TokenAlreadyRevokedError - токен отозван и окно его действия истекло (возможна компрометация);
//   - ErrEventStorageNoRecordFound - токен не найден.
func (re *AuthTokenPostgres) RevokeRefresh(ctx context.Context, refreshToken string, grace time.Duration) (row dto.UserScopes, isRetried bool, err error) {
	// атомарный перевод действующего refresh токена в статус "отозван";
	// предотвращает повторную ротацию при гонке запросов
	updateSQL := `
		UPDATE
			` + re.table.Name + `
		SET
			token_status = $4,
			expires_at = $5
		WHERE
			` + re.table.PrimaryKey + ` = $1 AND token_type = $2 AND token_status = $3 AND expires_at > NOW()
		RETURNING
			user_id,
			session_id,
			token_scopes;`

	var (
		userID    uuid.UUID
		sessionID uint32
	)

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		updateSQL,
		refreshToken,
		authtokentype.Refresh,
		authtokenstatus.Enabled,
		authtokenstatus.Revoked,
		time.Now().Add(grace),
	).Scan(
		&userID,
		&sessionID,
		&row,
	)
	if err != nil {
		// если токен не существует (обновление не произошло) - то он может быть уже отозван или истёк срок его действия
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return re.fetchEnabledRefreshToken(ctx, refreshToken)
		}

		return dto.UserScopes{}, false, re.errorWrapper.Wrap(err)
	}

	row.UserID = userID
	row.SessionID = sessionID

	return row, false, nil
}

// fetchEnabledRefreshToken - возвращает токен, если он ещё действующий или отозванный, но его окно действия ещё не закрыто.
func (re *AuthTokenPostgres) fetchEnabledRefreshToken(ctx context.Context, refreshToken string) (row dto.UserScopes, isRetried bool, err error) {
	sql := `
		SELECT
			user_id,
			session_id,
			token_scopes,
			token_status,
			(expires_at <= NOW()) as is_expired
		FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = $1 AND token_type = $2
		LIMIT 1;`

	var (
		userID    uuid.UUID
		sessionID uint32
		status    authtokenstatus.Enum
		isExpired bool
	)

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		refreshToken,
		authtokentype.Refresh,
	).Scan(
		&userID,
		&sessionID,
		&row,
		&status,
		&isExpired,
	)
	if err != nil {
		return dto.UserScopes{}, false, re.errorWrapper.Wrap(err)
	}

	if isExpired {
		if status == authtokenstatus.Enabled {
			return dto.UserScopes{}, false, ErrTokenExpired
		}

		return row, false, NewTokenAlreadyRevokedError(userID, sessionID)
	}

	row.UserID = userID
	row.SessionID = sessionID

	return row, status == authtokenstatus.Revoked, nil
}

// FetchLastEnabledPairBySessionID - возвращает последнюю активную пару токенов указанной
// сессии для идемпотентного ответа: действующий refresh токен (он всегда один на сессию) и
// последний действующий access токен. Access токен для JWT не хранится в БД,
// в этом случае access токен возвращается пустым.
func (re *AuthTokenPostgres) FetchLastEnabledPairBySessionID(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uint32,
) (access, refresh entity.AuthToken, err error) {
	refresh, err = re.fetchActiveToken(ctx, userID, sessionID, authtokentype.Refresh)
	if err != nil {
		return entity.AuthToken{}, entity.AuthToken{}, err
	}

	access, err = re.fetchActiveToken(ctx, userID, sessionID, authtokentype.Access)
	if err != nil {
		if !errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return entity.AuthToken{}, entity.AuthToken{}, err
		}

		// access токен для JWT может отсутствовать в БД
		return entity.AuthToken{}, refresh, nil
	}

	return access, refresh, nil
}

// fetchActiveToken - возвращает последний действующий токен сессии указанного типа.
func (re *AuthTokenPostgres) fetchActiveToken(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uint32,
	tokenType authtokentype.Enum,
) (row entity.AuthToken, err error) {
	sql := `
		SELECT
			` + re.table.PrimaryKey + `,
			token_scopes,
			expires_at
		FROM
			` + re.table.Name + `
		WHERE
			user_id = $1 AND session_id = $2 AND token_type = $3 AND token_status = $4
		ORDER BY
			created_at DESC
		LIMIT 1;`

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		userID,
		sessionID,
		tokenType,
		authtokenstatus.Enabled,
	).Scan(
		&row.Token,
		&row.Scopes,
		&row.ExpiresAt,
	)
	if err != nil {
		return entity.AuthToken{}, re.errorWrapper.Wrap(err)
	}

	row.Type = tokenType
	row.UserID = userID
	row.SessionID = sessionID

	return row, nil
}

// RevokeSessionByRefreshToken - отзывает все действующие токены сессии,
// которой принадлежит указанный refresh токен (logout).
func (re *AuthTokenPostgres) RevokeSessionByRefreshToken(ctx context.Context, refreshToken string) error {
	sql := `
        UPDATE
            ` + re.table.Name + ` t
        SET
			token_status = $4,
			expires_at = NOW()
		FROM
			(
				SELECT user_id, session_id
				FROM ` + re.table.Name + `
				WHERE ` + re.table.PrimaryKey + ` = $1 AND token_type = $2
				LIMIT 1
			) s
        WHERE
            t.user_id = s.user_id AND t.session_id = s.session_id AND t.token_status = $3;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		refreshToken,
		authtokentype.Refresh,
		authtokenstatus.Enabled,
		authtokenstatus.Revoked,
	)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
			err = errors.ErrEventStorageNoRecordFound
		}

		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// RevokeSession - отзывает все действующие токены указанной сессии пользователя
// (используется при обнаружении повторного использования отозванного refresh токена).
func (re *AuthTokenPostgres) RevokeSession(ctx context.Context, userID uuid.UUID, sessionID uint32) error {
	sql := `
        UPDATE
            ` + re.table.Name + `
        SET
			token_status = $4,
			expires_at = NOW()
        WHERE
            user_id = $1 AND session_id = $2 AND token_status = $3;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		userID,
		sessionID,
		authtokenstatus.Enabled,
		authtokenstatus.Revoked,
	)
	// если это внутренняя ошибка
	if err != nil && !errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
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
	if err != nil && !errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

type (
	// TokenAlreadyRevokedError - ошибка, когда токен уже отозван.
	// TODO: перенести в общее хранилище ошибок пакета.
	TokenAlreadyRevokedError struct {
		UserID    uuid.UUID
		SessionID uint32
	}
)

// NewTokenAlreadyRevokedError - создаёт ошибку TokenAlreadyRevokedError для указанного типа параметра.
func NewTokenAlreadyRevokedError(userID uuid.UUID, sessionID uint32) error {
	return &TokenAlreadyRevokedError{
		UserID:    userID,
		SessionID: sessionID,
	}
}

func (e *TokenAlreadyRevokedError) Error() string {
	return "token is already revoked"
}
