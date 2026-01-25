package service

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum/settingtype"
	"github.com/mondegor/go-components/mrsettings/field"
)

type (
	// SettingsGetter - компонент для извлечения настроек, которые хранятся в хранилище данных.
	SettingsGetter struct {
		parser       field.ValueParser
		storage      settingsGetterStorage
		errorWrapper errors.Wrapper
	}

	settingsGetterStorage interface {
		FetchOne(ctx context.Context, id uint64) (entity.Setting, error)
	}
)

// NewSettingsGetter - создаёт объект SettingsGetter.
func NewSettingsGetter(
	parser field.ValueParser,
	storage settingsGetterStorage,
) *SettingsGetter {
	return &SettingsGetter{
		parser:       parser,
		storage:      storage,
		errorWrapper: errors.NewServiceWrapper(),
	}
}

// Get - возвращает строковое значение настройки с указанным идентификатором.
func (sv *SettingsGetter) Get(ctx context.Context, id uint64) (string, error) {
	value, err := sv.getValue(ctx, id, settingtype.String)
	if err != nil {
		return "", err
	}

	parsedValue, err := sv.parser.ParseString(value)
	if err != nil {
		return "", sv.errorWrapper.Wrap(err, "itemId", id)
	}

	return parsedValue, nil
}

// GetList - возвращает список строковых значений настройки с указанным идентификатором.
func (sv *SettingsGetter) GetList(ctx context.Context, id uint64) ([]string, error) {
	value, err := sv.getValue(ctx, id, settingtype.StringList)
	if err != nil {
		return nil, err
	}

	parsedValue, err := sv.parser.ParseStringList(value)
	if err != nil {
		return nil, sv.errorWrapper.Wrap(err, "itemId", id)
	}

	return parsedValue, nil
}

// GetInt64 - возвращает целое знаковое значения настройки с указанным идентификатором.
func (sv *SettingsGetter) GetInt64(ctx context.Context, id uint64) (int64, error) {
	value, err := sv.getValue(ctx, id, settingtype.Integer)
	if err != nil {
		return 0, err
	}

	parsedValue, err := sv.parser.ParseInt64(value)
	if err != nil {
		return 0, sv.errorWrapper.Wrap(err, "itemId", id)
	}

	return parsedValue, nil
}

// GetInt64List - возвращает список целых знаковых значений настройки с указанным идентификатором.
func (sv *SettingsGetter) GetInt64List(ctx context.Context, id uint64) ([]int64, error) {
	value, err := sv.getValue(ctx, id, settingtype.IntegerList)
	if err != nil {
		return nil, err
	}

	parsedValue, err := sv.parser.ParseInt64List(value)
	if err != nil {
		return nil, sv.errorWrapper.Wrap(err, "itemId", id)
	}

	return parsedValue, nil
}

// GetBool - возвращает булево значение настройки с указанным идентификатором.
func (sv *SettingsGetter) GetBool(ctx context.Context, id uint64) (bool, error) {
	value, err := sv.getValue(ctx, id, settingtype.Boolean)
	if err != nil {
		return false, err
	}

	parsedValue, err := sv.parser.ParseBool(value)
	if err != nil {
		return false, sv.errorWrapper.Wrap(err, "itemId", id)
	}

	return parsedValue, nil
}

func (sv *SettingsGetter) getValue(ctx context.Context, id uint64, rowType settingtype.Enum) (string, error) {
	row, err := sv.storage.FetchOne(ctx, id)
	if err != nil {
		return "", sv.errorWrapper.Wrap(err, "itemId", id)
	}

	if row.Type != rowType && rowType != settingtype.String {
		return "", errors.ErrInternalInvalidType.New(
			"type", rowType,
			"expected", row.Type,
		)
	}

	return row.Value, nil
}
