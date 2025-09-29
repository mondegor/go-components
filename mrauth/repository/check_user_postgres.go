package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrpostgres/db"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"

	core "github.com/mondegor/go-components/internal"
)

type (
	// CheckUserPostgres - репозиторий для хранения сообщений подготовленных для отправки различным получателям.
	CheckUserPostgres struct {
		client            mrstorage.DBConnManager
		table             mrsql.DBTableInfo
		repoUserIDByEmail db.FieldFetcher[string, uuid.UUID]
		repoUserIDByPhone db.FieldFetcher[uint64, uuid.UUID]
		errorWrapper      core.ErrorWrapper
	}
)

// NewCheckUserPostgres - создаёт объект UserPostgres.
func NewCheckUserPostgres(client mrstorage.DBConnManager, table mrsql.DBTableInfo) *CheckUserPostgres {
	return &CheckUserPostgres{
		client: client,
		table:  table,
		repoUserIDByEmail: db.NewFieldFetcher[string, uuid.UUID](
			client,
			table.Name,
			"user_email",
			"user_id",
			"deleted_at",
		),
		repoUserIDByPhone: db.NewFieldFetcher[uint64, uuid.UUID](
			client,
			table.Name,
			"user_phone",
			"user_id",
			"deleted_at",
		),
		errorWrapper: core.NewStorageErrorWrapper(table.Name),
	}
}

// UserIDByEmail - возвращает список сообщений по их указанным SettingID.
func (re *CheckUserPostgres) UserIDByEmail(ctx context.Context, userEmail string) (rowID uuid.UUID, err error) {
	rowID, err = re.repoUserIDByEmail.Fetch(ctx, userEmail)
	if err != nil {
		return uuid.Nil, re.errorWrapper.WrapError(err)
	}

	return rowID, nil
}

// UserIDByPhone - возвращает список сообщений по их указанным SettingID.
func (re *CheckUserPostgres) UserIDByPhone(ctx context.Context, userPhone uint64) (rowID uuid.UUID, err error) {
	rowID, err = re.repoUserIDByPhone.Fetch(ctx, userPhone)
	if err != nil {
		return uuid.Nil, re.errorWrapper.WrapError(err)
	}

	return rowID, nil
}
