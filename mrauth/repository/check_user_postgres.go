package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrpostgres/db"
	"github.com/mondegor/go-sysmess/mrstorage"
)

type (
	// CheckUserPostgres - comment struct.
	CheckUserPostgres struct {
		client            mrstorage.DBConnManager
		errorWrapper      errors.Wrapper
		tableName         string
		repoUserIDByEmail db.FieldFetcher[string, uuid.UUID]
		repoUserIDByPhone db.FieldFetcher[uint64, uuid.UUID]
	}
)

// NewCheckUserPostgres - создаёт объект UserPostgres.
func NewCheckUserPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *CheckUserPostgres {
	return &CheckUserPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
		repoUserIDByEmail: db.NewFieldFetcher[string, uuid.UUID](
			client,
			tableName,
			"user_email",
			"user_id",
			"deleted_at",
		),
		repoUserIDByPhone: db.NewFieldFetcher[uint64, uuid.UUID](
			client,
			tableName,
			"user_phone",
			"user_id",
			"deleted_at",
		),
	}
}

// UserIDByEmail - возвращает список сообщений по их указанным ID.
func (re *CheckUserPostgres) UserIDByEmail(ctx context.Context, userEmail string) (rowID uuid.UUID, err error) {
	rowID, err = re.repoUserIDByEmail.Fetch(ctx, userEmail)
	if err != nil {
		return uuid.Nil, re.errorWrapper.Wrap(err)
	}

	return rowID, nil
}

// UserIDByPhone - возвращает список сообщений по их указанным ID.
func (re *CheckUserPostgres) UserIDByPhone(ctx context.Context, userPhone uint64) (rowID uuid.UUID, err error) {
	rowID, err = re.repoUserIDByPhone.Fetch(ctx, userPhone)
	if err != nil {
		return uuid.Nil, re.errorWrapper.Wrap(err)
	}

	return rowID, nil
}
