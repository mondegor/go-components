package repository

import (
	"context"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"

	core "github.com/mondegor/go-components/internal"
)

const (
	defaultTableNameLog = "settings_log"
)

type (
	// SettingLogPostgres - репозиторий для хранения элементов настроек.
	SettingLogPostgres struct {
		client       mrstorage.DBConnManager
		tableName    string
		tableSource  mrsql.DBTableInfo
		errorWrapper core.ErrorWrapper
	}
)

// NewSettingLogPostgres - создаёт объект SettingLogPostgres.
func NewSettingLogPostgres(
	client mrstorage.DBConnManager,
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
		tableName:    tableName,
		tableSource:  tableSource,
		errorWrapper: core.NewStorageErrorWrapper(tableName),
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
