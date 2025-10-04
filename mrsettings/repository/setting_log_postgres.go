package repository

import (
	"context"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
)

const (
	defaultTableNameLog = "settings_log"
)

type (
	// SettingLogPostgres - репозиторий для хранения элементов настроек.
	SettingLogPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper mrerr.ErrorWrapper
		tableName    string
		tableSource  mrsql.DBTableInfo
	}
)

// NewSettingLogPostgres - создаёт объект SettingLogPostgres.
func NewSettingLogPostgres(
	client mrstorage.DBConnManager,
	errorWrapper mrerr.ErrorWrapper,
	tableName string,
	tableSource mrsql.DBTableInfo,
) *SettingLogPostgres {
	if tableName == "" {
		tableName = defaultTableNameLog
	}

	if tableSource.Name == "" {
		tableSource.Name = defaultTableName
	}

	if tableSource.PrimaryKey == "" {
		tableSource.PrimaryKey = defaultPrimaryKey
	}

	return &SettingLogPostgres{
		client:       client,
		errorWrapper: mrerr.NewErrorWrapper(errorWrapper, tableName),
		tableName:    tableName,
		tableSource:  tableSource,
	}
}

// Insert - фиксирует изменение настройки.
func (re *SettingLogPostgres) Insert(ctx context.Context, settingID uint64, newValue string) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				` + re.tableSource.PrimaryKey + `,
				setting_name,
				setting_new_value,
				setting_old_value,
				created_at
			)
		SELECT
			$1,
			setting_name,
			$2,
			setting_value,
			NOW()
		FROM
			` + re.tableName + `
		WHERE
			` + re.tableSource.PrimaryKey + ` = $1;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		settingID,
		newValue,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err, "storage-data", mrargs.Group{"id": settingID})
	}

	return nil
}
