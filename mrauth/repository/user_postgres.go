package repository

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrpostgres/db"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
)

type (
	// UserPostgres - comment struct.
	UserPostgres struct {
		client            mrstorage.DBConnManager
		table             mrsql.DBTableInfo
		repoUserIDByEmail db.FieldFetcher[string, uuid.UUID]
		repoUserIDByPhone db.FieldFetcher[uint64, uuid.UUID]
		repoEmail         db.FieldUpdater[uuid.UUID, string]
		repoPhone         db.FieldUpdater[uuid.UUID, uint64]
		errorWrapper      mrerr.ErrorWrapper
	}
)

// NewUserPostgres - создаёт объект UserPostgres.
func NewUserPostgres(
	client mrstorage.DBConnManager,
	errorWrapper mrerr.ErrorWrapper,
	table mrsql.DBTableInfo,
) *UserPostgres {
	return &UserPostgres{
		client:       client,
		errorWrapper: mrerr.NewErrorWrapper(errorWrapper, table.Name),
		table:        table,
		repoEmail: db.NewFieldUpdater[uuid.UUID, string](
			client,
			table.Name,
			table.PrimaryKey,
			"user_email",
			"deleted_at",
		),
		repoPhone: db.NewFieldUpdater[uuid.UUID, uint64](
			client,
			table.Name,
			table.PrimaryKey,
			"user_phone",
			"deleted_at",
		),
	}
}

// FetchOne - возвращает список сообщений по их указанным SettingID.
func (re *UserPostgres) FetchOne(ctx context.Context, userID uuid.UUID) (row entity.User, err error) {
	return re.fetchOneBy(ctx, "user_id", userID)
}

// FetchOneByLogin - возвращает список сообщений по их указанным SettingID.
func (re *UserPostgres) FetchOneByLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (row entity.User, err error) {
	if userLogin.Type == addresstype.Email {
		return re.fetchOneBy(ctx, "user_email", userLogin.Value)
	}

	if userLogin.Type == addresstype.Phone {
		userLoginPhone, err := strconv.ParseUint(userLogin.Value, 10, 64)
		if err != nil {
			return entity.User{}, mr.ErrInternal.Wrap(errors.New("userLoginPhone is incorrect"))
		}

		return re.fetchOneBy(ctx, "user_phone", userLoginPhone)
	}

	return entity.User{}, mr.ErrInternal.Wrap(errors.New("userLogin is incorrect"))
}

func (re *UserPostgres) fetchOneBy(ctx context.Context, fieldName string, fieldValue any) (row entity.User, err error) {
	sql := `
		SELECT
			user_id,
			user_email,
			user_phone,
			lang_code,
			user_status
		FROM
			` + re.table.Name + `
		WHERE
			` + fieldName + ` = $1
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
		&row.Status,
	)
	if err != nil {
		return entity.User{}, re.errorWrapper.WrapError(err)
	}

	// from nullable user_phone field
	if userPhone != nil {
		row.Phone = *userPhone
	}

	return row, nil
}

// UserIDByEmail - возвращает список сообщений по их указанным SettingID.
func (re *UserPostgres) UserIDByEmail(ctx context.Context, userEmail string) (rowID uuid.UUID, err error) {
	rowID, err = re.repoUserIDByEmail.Fetch(ctx, userEmail)
	if err != nil {
		return uuid.Nil, re.errorWrapper.WrapError(err)
	}

	return rowID, nil
}

// UserIDByPhone - возвращает список сообщений по их указанным SettingID.
func (re *UserPostgres) UserIDByPhone(ctx context.Context, userPhone uint64) (rowID uuid.UUID, err error) {
	rowID, err = re.repoUserIDByPhone.Fetch(ctx, userPhone)
	if err != nil {
		return uuid.Nil, re.errorWrapper.WrapError(err)
	}

	return rowID, nil
}

// Insert - возвращает список сообщений по их указанным SettingID.
func (re *UserPostgres) Insert(ctx context.Context, row entity.User) (rowID uuid.UUID, err error) {
	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				user_id,
				user_email,
				user_phone,
				lang_code,
				user_status
			)
		VALUES
			(gen_random_uuid(), $1, $2, $3, $4)
        RETURNING
            user_id;`

	var userPhone *uint64

	// to nullable user_phone field
	if row.Phone != 0 {
		userPhone = &row.Phone
	}

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		row.Email,
		userPhone,
		row.LangCode,
		row.Status,
	).Scan(
		&row.ID,
	)
	if err != nil {
		return uuid.Nil, re.errorWrapper.WrapError(err)
	}

	return row.ID, nil
}

// UpdateEmail - comments method.
func (re *UserPostgres) UpdateEmail(ctx context.Context, userID uuid.UUID, value string) error {
	return re.repoEmail.Update(ctx, userID, value)
}

// UpdatePhone - comments method.
func (re *UserPostgres) UpdatePhone(ctx context.Context, userID uuid.UUID, value uint64) error {
	return re.repoPhone.Update(ctx, userID, value)
}
