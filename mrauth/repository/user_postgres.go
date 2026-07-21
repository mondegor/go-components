package repository

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrpostgres/db"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
)

type (
	// UserPostgres - хранилище пользователей в PostgreSQL.
	UserPostgres struct {
		client       mrstorage.DBConnManager
		tableName    string
		repoEmail    db.FieldUpdater[uuid.UUID, string]
		repoPhone    db.FieldUpdater[uuid.UUID, uint64]
		errorWrapper errors.Wrapper
	}
)

// NewUserPostgres - создаёт объект UserPostgres.
func NewUserPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *UserPostgres {
	return &UserPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
		repoEmail: db.NewFieldUpdater[uuid.UUID, string](
			client,
			tableName,
			"user_id",
			"user_email",
			"deleted_at",
		),
		repoPhone: db.NewFieldUpdater[uuid.UUID, uint64](
			client,
			tableName,
			"user_id",
			"user_phone",
			"deleted_at",
		),
	}
}

// FetchOne - возвращает пользователя по его идентификатору.
func (re *UserPostgres) FetchOne(ctx context.Context, userID uuid.UUID) (row entity.User, err error) {
	return re.fetchOneBy(ctx, "user_id", userID)
}

// FetchOneByLogin - возвращает пользователя по его логину (email или телефон).
func (re *UserPostgres) FetchOneByLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (row entity.User, err error) {
	if userLogin.Is(addresstype.Email) {
		return re.fetchOneBy(ctx, "user_email", userLogin.Value())
	}

	if userLogin.Is(addresstype.Phone) {
		userLoginPhone, err := strconv.ParseUint(userLogin.Value(), 10, 64)
		if err != nil {
			return entity.User{}, errors.NewInternalError("userLoginPhone is incorrect")
		}

		return re.fetchOneBy(ctx, "user_phone", userLoginPhone)
	}

	return entity.User{}, errors.NewInternalError("userLogin is incorrect")
}

func (re *UserPostgres) fetchOneBy(ctx context.Context, fieldName string, fieldValue any) (row entity.User, err error) {
	sql := `
		SELECT
			user_id,
			user_email,
			user_phone,
			lang_code,
			user_timezone,
			user_status,
			created_at,
			updated_at
		FROM
			` + re.tableName + `
		WHERE
			` + fieldName + ` = $1 AND deleted_at IS NULL
		LIMIT 1;`

	var userPhone *uint64

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		fieldValue,
	).Scan(
		&row.ID,
		&row.Email,
		&userPhone,
		&row.LangCode,
		&row.TimeZone,
		&row.Status,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		return entity.User{}, re.errorWrapper.Wrap(err)
	}

	// from nullable user_phone field
	if userPhone != nil {
		row.Phone = *userPhone
	}

	// системное время: домен всегда оперирует UTC независимо от зоны сессии БД
	row.CreatedAt = row.CreatedAt.UTC()
	row.UpdatedAt = row.UpdatedAt.UTC()

	return row, nil
}

// Insert - добавляет нового пользователя и возвращает сгенерированный идентификатор.
func (re *UserPostgres) Insert(ctx context.Context, row entity.ExtendedUser) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				user_email,
				user_phone,
				lang_code,
				user_timezone,
				registered_ip,
				registered_proxy_ip,
				user_status
			)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8);`

	var userPhone *uint64

	// to nullable user_phone field
	if row.Phone != 0 {
		userPhone = &row.Phone
	}

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.ID,
		row.Email,
		userPhone,
		row.LangCode,
		row.TimeZone,
		row.RegisteredIP.Real,
		row.RegisteredIP.Proxy,
		row.Status,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// UpdateEmail - обновляет email пользователя.
func (re *UserPostgres) UpdateEmail(ctx context.Context, userID uuid.UUID, value string) error {
	return re.repoEmail.Update(ctx, userID, value)
}

// UpdatePhone - обновляет телефон пользователя.
func (re *UserPostgres) UpdatePhone(ctx context.Context, userID uuid.UUID, value uint64) error {
	return re.repoPhone.Update(ctx, userID, value)
}

// UpdateSettings - обновляет язык (локаль) и часовой пояс пользователя.
func (re *UserPostgres) UpdateSettings(ctx context.Context, row entity.UserSettings) error {
	sql := `
		UPDATE
			` + re.tableName + `
		SET
			lang_code = $2,
			user_timezone = $3,
			updated_at = NOW()
		WHERE
			user_id = $1 AND deleted_at IS NULL;`

	if err := re.client.Conn(ctx).ExecRow(ctx, sql, row.UserID, row.LangCode, row.TimeZone); err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
