package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/authtokenstatus"
	"github.com/mondegor/go-components/mrauth/enum/authtokentype"
)

type (
	// AuthTokenPostgres - хранилище токенов авторизации в PostgreSQL.
	AuthTokenPostgres struct {
		client                     mrstorage.DBConnManager
		errorWrapper               errors.Wrapper
		tableName                  string
		whereRefreshTokenAndStatus string // расчитывается заранее для эффективной работы планировщика Postgres
	}
)

// NewAuthTokenPostgres - создаёт объект AuthTokenPostgres.
func NewAuthTokenPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *AuthTokenPostgres {
	return &AuthTokenPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
		whereRefreshTokenAndStatus: " AND token_type = " + strconv.FormatUint(uint64(authtokentype.Refresh), 10) +
			" AND token_status = " +
			strconv.FormatUint(uint64(authtokenstatus.Enabled), 10),
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
			` + re.tableName + `
		WHERE
			auth_token = $1 AND token_type = $2 AND token_status = $3
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

// FetchOpenSessionCount - возвращает число открытых сессий пользователя в указанном realm
// (сессия открыта = есть действующий не истёкший refresh токен). Лимит сессий скоупится
// по (user_id, realm), поэтому счёт ведётся в пределах одного realm.
func (re *AuthTokenPostgres) FetchOpenSessionCount(ctx context.Context, userID uuid.UUID, realmID uint16) (count int, err error) {
	sql := `
		SELECT
			COUNT(DISTINCT session_id)
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1 AND realm_id = $2` + re.whereRefreshTokenAndStatus + ` AND expires_at > NOW();`

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		userID,
		realmID,
	).Scan(&count)
	if err != nil {
		return 0, re.errorWrapper.Wrap(err)
	}

	return count, nil
}

// FetchOpenSessionIDs - возвращает идентификаторы открытых сессий пользователя в указанном realm,
// отсортированные по возрасту действующего refresh токена (created_at, по возрастанию):
// первыми идут сессии, refresh токен которых дольше всех не обновлялся (наименее активные).
// Открыта = есть действующий не истёкший refresh токен. Скоуп по realm согласован с
// FetchOpenSessionCount: лимит сессий считается и обрезается в пределах одного realm.
func (re *AuthTokenPostgres) FetchOpenSessionIDs(ctx context.Context, userID uuid.UUID, realmID uint16) (rows []uint32, err error) {
	// GROUP BY используется на всякий случай, так как на сессию приходится ровно один enabled refresh токен;
	// MIN(created_at) устойчив к такому "на всякий случай" дубликату
	sql := `
		SELECT
			session_id
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1 AND realm_id = $2` + re.whereRefreshTokenAndStatus + ` AND expires_at > NOW()
		GROUP BY
			session_id
		ORDER BY
			MIN(created_at), session_id;`

	cursor, err := re.client.Conn(ctx).Query(
		ctx,
		sql,
		userID,
		realmID,
	)
	if err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	defer cursor.Close()

	rows = make([]uint32, 0)

	for cursor.Next() {
		var sessionID uint32

		if err = cursor.Scan(&sessionID); err != nil {
			return nil, re.errorWrapper.Wrap(err)
		}

		rows = append(rows, sessionID)
	}

	if err = cursor.Err(); err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	return rows, nil
}

// Insert - сохраняет несколько токенов авторизации.
func (re *AuthTokenPostgres) Insert(ctx context.Context, rows []entity.AuthToken) error {
	if len(rows) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(rows))
	tokenTypes := make([]int16, 0, len(rows))
	userIDs := make([]uuid.UUID, 0, len(rows))
	realmIDs := make([]uint16, 0, len(rows))
	sessionIDs := make([]uint32, 0, len(rows))
	scopes := make([]entity.AuthTokenScopes, 0, len(rows))
	expiresAts := make([]time.Time, 0, len(rows))

	for _, row := range rows {
		tokens = append(tokens, row.Token)
		tokenTypes = append(tokenTypes, int16(row.Type))
		userIDs = append(userIDs, row.UserID)
		realmIDs = append(realmIDs, row.RealmID)
		sessionIDs = append(sessionIDs, row.SessionID)
		scopes = append(scopes, row.Scopes) // userID/sessionID будут исключены, т.к. они сохраняются в отдельных полях
		expiresAts = append(expiresAts, row.ExpiresAt)
	}

	sql := `
		INSERT INTO ` + re.tableName + `
			(
				auth_token,
				token_type,
				user_id,
				realm_id,
				session_id,
				token_scopes,
				token_status,
				expires_at
			)
		SELECT token, token_type, user_id, realm_id, session_id, token_scopes, $8, expires_at
		FROM
			UNNEST($1::varchar[], $2::int2[], $3::uuid[], $4::int4[], $5::int8[], $6::jsonb[], $7::timestamptz[])
			as t(token, token_type, user_id, realm_id, session_id, token_scopes, expires_at);`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		tokens,
		tokenTypes,
		userIDs,
		realmIDs,
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
// окно действия длиной grace, в течение которого допустим повторный запрос с этим же токеном.
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
			` + re.tableName + `
		SET
			token_status = $4,
			expires_at = $5
		WHERE
			auth_token = $1 AND token_type = $2 AND token_status = $3 AND expires_at > NOW()
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
			` + re.tableName + `
		WHERE
			auth_token = $1 AND token_type = $2
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
// сессии: действующий refresh токен (он всегда один на сессию) и
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
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return entity.AuthToken{}, refresh, nil
		}

		return entity.AuthToken{}, entity.AuthToken{}, err
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
			auth_token,
			token_scopes,
			expires_at
		FROM
			` + re.tableName + `
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

	// запрос не фильтрует по expires_at, поэтому действующий токен мог уже истечь;
	// в норме такого быть не должно, проверка защищает от выдачи просроченного токена
	if row.ExpiresAt.Before(time.Now()) {
		return entity.AuthToken{}, ErrTokenExpired
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
            ` + re.tableName + ` t
        SET
			token_status = $4,
			expires_at = NOW()
		FROM
			(
				SELECT user_id, session_id
				FROM ` + re.tableName + `
				WHERE auth_token = $1 AND token_type = $2
				LIMIT 1
			) s
        WHERE
            t.user_id = s.user_id AND t.session_id = s.session_id AND t.token_status = $3;`

	_, err := re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		refreshToken,
		authtokentype.Refresh,
		authtokenstatus.Enabled,
		authtokenstatus.Revoked,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// RevokeTokensBySessionID - отзывает все действующие токены указанной сессии пользователя
// (используется при обнаружении повторного использования отозванного refresh токена).
func (re *AuthTokenPostgres) RevokeTokensBySessionID(ctx context.Context, userID uuid.UUID, sessionID uint32) error {
	return re.RevokeTokensBySessionIDs(ctx, userID, []uint32{sessionID})
}

// RevokeTokensBySessionIDs - отзывает все действующие токены указанных сессий пользователя (идемпотентно:
// отсутствие подходящих токенов не считается ошибкой). Используется при закрытии сессий по их списку.
func (re *AuthTokenPostgres) RevokeTokensBySessionIDs(ctx context.Context, userID uuid.UUID, sessionIDs []uint32) error {
	if len(sessionIDs) == 0 {
		return nil
	}

	sql := `
        UPDATE
            ` + re.tableName + `
        SET
			token_status = $4,
			expires_at = NOW()
        WHERE
            user_id = $1 AND session_id = ANY($2::int8[]) AND token_status = $3;`

	_, err := re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		userID,
		sessionIDs,
		authtokenstatus.Enabled,
		authtokenstatus.Revoked,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// DeleteExpiredNonRefresh - удаляет пачку истёкших не-refresh токенов (access/API)
// (не более limit за один вызов) и возвращает число фактически удалённых строк.
// Счётчик нужен вызывающему как сигнал "пачка была полной, есть ещё".
// Очистка рассчитана на single-pod-планировщик (см. wire/mrauth/scheduler.NewService).
// FOR UPDATE SKIP LOCKED оставлен как защита от случайного двойного запуска, а не как поддержка мульти-пода.
func (re *AuthTokenPostgres) DeleteExpiredNonRefresh(ctx context.Context, limit int) (count int, err error) {
	sql := `
		DELETE FROM
			` + re.tableName + ` t1
		USING (
			SELECT
				auth_token
			FROM
				` + re.tableName + `
			WHERE
				expires_at < NOW() AND token_type <> $1
			ORDER BY
				expires_at ASC
			` + mrstorage.NonZeroLimit(limit) + `
			FOR UPDATE SKIP LOCKED
		) ei
		WHERE
			t1.auth_token = ei.auth_token;`

	count, err = re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		authtokentype.Refresh,
	)
	if err != nil {
		return 0, re.errorWrapper.Wrap(err)
	}

	return count, nil
}

// DeleteExpiredRefresh - удаляет пачку истёкших refresh токенов (не более limit за вызов)
// и возвращает сессии удалённых токенов как кандидатов на удаление (по одной записи на
// удалённый токен, без дедупликации - её делает Enqueue через ON CONFLICT). Длина результата
// равна числу удалённых токенов и служит вызывающему сигналом "пачка была полной, есть ещё".
// Очистка рассчитана на single-pod-планировщик (см. wire/mrauth/scheduler.NewService).
// FOR UPDATE SKIP LOCKED оставлен как защита от случайного двойного запуска, а не как поддержка мульти-пода.
func (re *AuthTokenPostgres) DeleteExpiredRefresh(ctx context.Context, limit int) (candidates []entity.SessionPK, err error) {
	sql := `
		WITH expired_items as (
			SELECT
			  	auth_token
			FROM
			  	` + re.tableName + `
			WHERE
				expires_at < NOW() AND token_type = $1
			ORDER BY
				expires_at ASC
		    ` + mrstorage.NonZeroLimit(limit) + `
			FOR UPDATE SKIP LOCKED
		),
		deleted_items as (
			DELETE FROM
				` + re.tableName + ` t1
			USING
				expired_items ei
			WHERE
				t1.auth_token = ei.auth_token
			RETURNING
				t1.user_id, t1.session_id
		)
		SELECT
			user_id, session_id
		FROM
			deleted_items;`

	cursor, err := re.client.Conn(ctx).Query(
		ctx,
		sql,
		authtokentype.Refresh,
	)
	if err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	defer cursor.Close()

	candidates = make([]entity.SessionPK, 0)

	for cursor.Next() {
		var pk entity.SessionPK

		if err = cursor.Scan(&pk.UserID, &pk.SessionID); err != nil {
			return nil, re.errorWrapper.Wrap(err)
		}

		candidates = append(candidates, pk)
	}

	if err = cursor.Err(); err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	return candidates, nil
}
