package getter

import (
	"context"

	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrsettings"
)

type (
	// Component - компонент для извлечения настроек,
	// которые хранятся в хранилище данных.
	Component struct {
		parser       mrsettings.ValueParser
		storage      mrsettings.Storage
		errorWrapper mrcore.UsecaseErrorWrapper
	}
)

// New - создаёт объект Component.
func New(parser mrsettings.ValueParser, storage mrsettings.Storage, errorWrapper mrcore.UsecaseErrorWrapper) *Component {
	return &Component{
		parser:       parser,
		storage:      storage,
		errorWrapper: errorWrapper,
	}
}

// Get - comment method.
func (co *Component) Get(ctx context.Context, id mrtype.KeyInt32) (string, error) {
	value, err := co.getValue(ctx, id, mrsettings.SettingTypeString)
	if err != nil {
		return "", err
	}

	parsedValue, err := co.parser.ParseString(value)
	if err != nil {
		return "", err
	}

	return parsedValue, nil
}

// GetList - comment method.
func (co *Component) GetList(ctx context.Context, id mrtype.KeyInt32) ([]string, error) {
	value, err := co.getValue(ctx, id, mrsettings.SettingTypeStringList)
	if err != nil {
		return nil, err
	}

	parsedValue, err := co.parser.ParseStringList(value)
	if err != nil {
		return nil, err
	}

	return parsedValue, nil
}

// GetInt64 - comment method.
func (co *Component) GetInt64(ctx context.Context, id mrtype.KeyInt32) (int64, error) {
	value, err := co.getValue(ctx, id, mrsettings.SettingTypeInteger)
	if err != nil {
		return 0, err
	}

	parsedValue, err := co.parser.ParseInt64(value)
	if err != nil {
		return 0, err
	}

	return parsedValue, nil
}

// GetInt64List - comment method.
func (co *Component) GetInt64List(ctx context.Context, id mrtype.KeyInt32) ([]int64, error) {
	value, err := co.getValue(ctx, id, mrsettings.SettingTypeIntegerList)
	if err != nil {
		return nil, err
	}

	parsedValue, err := co.parser.ParseInt64List(value)
	if err != nil {
		return nil, err
	}

	return parsedValue, nil
}

// GetBool - comment method.
func (co *Component) GetBool(ctx context.Context, id mrtype.KeyInt32) (bool, error) {
	value, err := co.getValue(ctx, id, mrsettings.SettingTypeBoolean)
	if err != nil {
		return false, err
	}

	parsedValue, err := co.parser.ParseBool(value)
	if err != nil {
		return false, err
	}

	return parsedValue, nil
}

func (co *Component) getValue(ctx context.Context, id mrtype.KeyInt32, rowType mrsettings.SettingType) (string, error) {
	row, err := co.storage.FetchOne(ctx, id)
	if err != nil {
		return "", co.errorWrapper.WrapErrorEntityNotFoundOrFailed(err, mrsettings.ModelNameEntitySetting, mrmsg.Data{"id": id})
	}

	if row.Type != rowType && rowType != mrsettings.SettingTypeString {
		return "", mrcore.ErrInternalInvalidType.New(rowType, row.Type)
	}

	return row.Value, nil
}
