package service

import (
	"context"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum/settingtype"
	"github.com/mondegor/go-components/mrsettings/field"
)

type (
	// SettingsSetter - компонент для сохранения настроек в хранилище данных.
	SettingsSetter struct {
		txManager    mrstorage.DBTxManager
		formatter    field.ValueFormatter
		storage      settingsSetterStorage
		storageLog   settingsLogStorage
		errorWrapper errors.Wrapper
	}

	settingsSetterStorage interface {
		Update(ctx context.Context, row entity.Setting) error
	}

	settingsLogStorage interface {
		Insert(ctx context.Context, settingID uint64, newValue string) error
	}
)

// NewSettingsSetter - создаёт объект SettingsSetter.
func NewSettingsSetter(
	txManager mrstorage.DBTxManager,
	formatter field.ValueFormatter,
	storage settingsSetterStorage,
	storageLog settingsLogStorage,
) *SettingsSetter {
	return &SettingsSetter{
		txManager:    txManager,
		formatter:    formatter,
		storage:      storage,
		storageLog:   storageLog,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Set - сохранение строкового значения настройки с указанным идентификатором.
func (sv *SettingsSetter) Set(ctx context.Context, id uint64, value string) error {
	formattedValue, err := sv.formatter.FormatString(value)
	if err != nil {
		return sv.errorWrapper.Wrap(err, "value", value)
	}

	return sv.setValue(ctx, id, formattedValue, settingtype.String)
}

// SetList - сохранение списка строковых значений настройки с указанным идентификатором.
func (sv *SettingsSetter) SetList(ctx context.Context, id uint64, value []string) error {
	formattedValue, err := sv.formatter.FormatStringList(value)
	if err != nil {
		return sv.errorWrapper.Wrap(err, "value", value)
	}

	return sv.setValue(ctx, id, formattedValue, settingtype.IntegerList)
}

// SetInt64 - сохранение целого знакового значения настройки с указанным идентификатором.
func (sv *SettingsSetter) SetInt64(ctx context.Context, id uint64, value int64) error {
	formattedValue, err := sv.formatter.FormatInt64(value)
	if err != nil {
		return sv.errorWrapper.Wrap(err, "value", value)
	}

	return sv.setValue(ctx, id, formattedValue, settingtype.Integer)
}

// SetInt64List - сохранение списка целых знаковых значений настройки с указанным идентификатором.
func (sv *SettingsSetter) SetInt64List(ctx context.Context, id uint64, value []int64) error {
	formattedValue, err := sv.formatter.FormatInt64List(value)
	if err != nil {
		return sv.errorWrapper.Wrap(err, "value", value)
	}

	return sv.setValue(ctx, id, formattedValue, settingtype.IntegerList)
}

// SetBool - сохранение булева значения настройки с указанным идентификатором.
func (sv *SettingsSetter) SetBool(ctx context.Context, id uint64, value bool) error {
	formattedValue, err := sv.formatter.FormatBool(value)
	if err != nil {
		return sv.errorWrapper.Wrap(err, "value", value)
	}

	return sv.setValue(ctx, id, formattedValue, settingtype.Boolean)
}

func (sv *SettingsSetter) setValue(ctx context.Context, id uint64, value string, rowType settingtype.Enum) error {
	row := entity.Setting{
		ID:    id,
		Type:  rowType,
		Value: value,
	}

	return sv.txManager.Do(ctx, func(ctx context.Context) error {
		if err := sv.storage.Update(ctx, row); err != nil {
			return sv.errorWrapper.Wrap(err, "itemId", id)
		}

		if err := sv.storageLog.Insert(ctx, id, value); err != nil {
			return sv.errorWrapper.Wrap(err, "itemId", id)
		}

		return nil
	})
}
