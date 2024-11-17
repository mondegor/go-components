package get

import (
	"context"

	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"

	"github.com/mondegor/go-components/mrsettings"
	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum"
)

type (
	// SettingsGetter - компонент для извлечения настроек, которые хранятся в хранилище данных.
	SettingsGetter struct {
		parser       mrsettings.ValueParser
		storage      mrsettings.Storage
		errorWrapper mrcore.UseCaseErrorWrapper
	}
)

// New - создаёт объект SettingsGetter.
func New(parser mrsettings.ValueParser, storage mrsettings.Storage, errorWrapper mrcore.UseCaseErrorWrapper) *SettingsGetter {
	return &SettingsGetter{
		parser:       parser,
		storage:      storage,
		errorWrapper: errorWrapper,
	}
}

// Get - возвращает строковое значение настройки с указанным идентификатором.
func (co *SettingsGetter) Get(ctx context.Context, id uint64) (string, error) {
	value, err := co.getValue(ctx, id, enum.SettingTypeString)
	if err != nil {
		return "", err
	}

	parsedValue, err := co.parser.ParseString(value)
	if err != nil {
		return "", err
	}

	return parsedValue, nil
}

// GetList - возвращает список строковых значений настройки с указанным идентификатором.
func (co *SettingsGetter) GetList(ctx context.Context, id uint64) ([]string, error) {
	value, err := co.getValue(ctx, id, enum.SettingTypeStringList)
	if err != nil {
		return nil, err
	}

	parsedValue, err := co.parser.ParseStringList(value)
	if err != nil {
		return nil, err
	}

	return parsedValue, nil
}

// GetInt64 - возвращает целое знаковое значения настройки с указанным идентификатором.
func (co *SettingsGetter) GetInt64(ctx context.Context, id uint64) (int64, error) {
	value, err := co.getValue(ctx, id, enum.SettingTypeInteger)
	if err != nil {
		return 0, err
	}

	parsedValue, err := co.parser.ParseInt64(value)
	if err != nil {
		return 0, err
	}

	return parsedValue, nil
}

// GetInt64List - возвращает список целых знаковых значений настройки с указанным идентификатором.
func (co *SettingsGetter) GetInt64List(ctx context.Context, id uint64) ([]int64, error) {
	value, err := co.getValue(ctx, id, enum.SettingTypeIntegerList)
	if err != nil {
		return nil, err
	}

	parsedValue, err := co.parser.ParseInt64List(value)
	if err != nil {
		return nil, err
	}

	return parsedValue, nil
}

// GetBool - возвращает булево значение настройки с указанным идентификатором.
func (co *SettingsGetter) GetBool(ctx context.Context, id uint64) (bool, error) {
	value, err := co.getValue(ctx, id, enum.SettingTypeBoolean)
	if err != nil {
		return false, err
	}

	parsedValue, err := co.parser.ParseBool(value)
	if err != nil {
		return false, err
	}

	return parsedValue, nil
}

func (co *SettingsGetter) getValue(ctx context.Context, id uint64, rowType enum.SettingType) (string, error) {
	row, err := co.storage.FetchOne(ctx, id)
	if err != nil {
		return "", co.errorWrapper.WrapErrorEntityNotFoundOrFailed(err, entity.ModelNameSetting, mrmsg.Data{"id": id})
	}

	if row.Type != rowType && rowType != enum.SettingTypeString {
		return "", mrcore.ErrInternalInvalidType.New(rowType, row.Type)
	}

	return row.Value, nil
}
