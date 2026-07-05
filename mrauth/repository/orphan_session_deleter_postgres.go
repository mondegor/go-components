package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/authtokenstatus"
	"github.com/mondegor/go-components/mrauth/enum/authtokentype"
)

type (
	// OrphanSessionDeleterPostgres - атомарно удаляет осиротевшие строки сессий (у которых не
	// осталось живого ENABLED непросроченного refresh-токена) из переданных кандидатов. Проверка
	// осиротелости и удаление выполняются ОДНИМ запросом (DELETE ... USING ... LEFT JOIN ...
	// WHERE t.auth_token IS NULL - anti-join), поэтому параллельный логин, переоткрывший сессию с
	// тем же (user_id, session_id) и выпустивший новый refresh, не приведёт к удалению уже живой
	// сессии. Кросс-табличная операция - знает имена обеих таблиц (sessions и auth_tokens).
	OrphanSessionDeleterPostgres struct {
		client              mrstorage.DBConnManager
		errorWrapper        errors.Wrapper
		sessionsTableName   string
		authTokensTableName string
	}
)

// NewOrphanSessionDeleterPostgres - создаёт объект OrphanSessionDeleterPostgres.
func NewOrphanSessionDeleterPostgres(
	client mrstorage.DBConnManager,
	sessionsTableName string,
	authTokensTableName string,
) *OrphanSessionDeleterPostgres {
	return &OrphanSessionDeleterPostgres{
		client:              client,
		errorWrapper:        errors.NewInfraStorageWrapper(),
		sessionsTableName:   sessionsTableName,
		authTokensTableName: authTokensTableName,
	}
}

// DeleteOrphaned - удаляет из переданных кандидатов те строки сессий, у которых не осталось ни
// одного живого (ENABLED && непросроченного) refresh-токена. Идемпотентна: отсутствующие и
// неосиротевшие строки игнорируются. Ротация не приводит к удалению - у живой сессии остаётся
// новый ENABLED-токен, и anti-join (LEFT JOIN … IS NULL) его находит в той же транзакции с удалением.
func (re *OrphanSessionDeleterPostgres) DeleteOrphaned(ctx context.Context, candidates []entity.SessionPK) error {
	if len(candidates) == 0 {
		return nil
	}

	userIDs := make([]uuid.UUID, 0, len(candidates))
	sessionIDs := make([]uint32, 0, len(candidates))

	for _, pk := range candidates {
		userIDs = append(userIDs, pk.UserID)
		sessionIDs = append(sessionIDs, pk.SessionID)
	}

	sql := `
		DELETE FROM
			` + re.sessionsTableName + ` s
		USING
			UNNEST($1::uuid[], $2::int8[]) as c(user_id, session_id)
		LEFT JOIN ` + re.authTokensTableName + ` t
			ON  t.user_id = c.user_id
			AND t.session_id = c.session_id
			AND t.token_type = $3
			AND t.token_status = $4
			AND t.expires_at > NOW()
		WHERE
			s.user_id = c.user_id
			AND s.session_id = c.session_id
			AND t.auth_token IS NULL;`

	_, err := re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		userIDs,
		sessionIDs,
		authtokentype.Refresh,
		authtokenstatus.Enabled,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
