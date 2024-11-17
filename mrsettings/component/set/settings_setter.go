package set

import (
	"context"

	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"

	"github.com/mondegor/go-components/mrsettings"
	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum"
)

type (
	// SettingsSetter - компонент для сохранения настроек в хранилище данных.
	SettingsSetter struct {
		formatter    mrsettings.ValueFormatter
		storage      mrsettings.Storage
		eventEmitter mrsender.EventEmitter
		errorWrapper mrcore.UseCaseErrorWrapper
	}
)

// New - создаёт объект SettingsSetter.
func New(
	formatter mrsettings.ValueFormatter,
	storage mrsettings.Storage,
	eventEmitter mrsender.EventEmitter,
	errorWrapper mrcore.UseCaseErrorWrapper,
) *SettingsSetter {
	return &SettingsSetter{
		formatter:    formatter,
		storage:      storage,
		eventEmitter: decorator.NewSourceEmitter(eventEmitter, entity.ModelNameSetting+".Setter"),
		errorWrapper: errorWrapper,
	}
}

// Set - сохранение строкового значения настройки с указанным идентификатором.
func (co *SettingsSetter) Set(ctx context.Context, id uint64, value string) error {
	formattedValue, err := co.formatter.FormatString(value)
	if err != nil {
		return err
	}

	if err = co.setValue(ctx, id, formattedValue, enum.SettingTypeString); err != nil {
		return err
	}

	co.eventEmitter.Emit(ctx, "Set", mrmsg.Data{"id": id})

	return nil
}

// SetList - сохранение списка строковых значений настройки с указанным идентификатором.
func (co *SettingsSetter) SetList(ctx context.Context, id uint64, value []string) error {
	formattedValue, err := co.formatter.FormatStringList(value)
	if err != nil {
		return err
	}

	if err = co.setValue(ctx, id, formattedValue, enum.SettingTypeIntegerList); err != nil {
		return err
	}

	co.eventEmitter.Emit(ctx, "SetList", mrmsg.Data{"id": id})

	return nil
}

// SetInt64 - сохранение целого знакового значения настройки с указанным идентификатором.
func (co *SettingsSetter) SetInt64(ctx context.Context, id uint64, value int64) error {
	formattedValue, err := co.formatter.FormatInt64(value)
	if err != nil {
		return err
	}

	if err = co.setValue(ctx, id, formattedValue, enum.SettingTypeInteger); err != nil {
		return err
	}

	co.eventEmitter.Emit(ctx, "SetInt64", mrmsg.Data{"id": id})

	return nil
}

// SetInt64List - сохранение списка целых знаковых значений настройки с указанным идентификатором.
func (co *SettingsSetter) SetInt64List(ctx context.Context, id uint64, value []int64) error {
	formattedValue, err := co.formatter.FormatInt64List(value)
	if err != nil {
		return err
	}

	if err = co.setValue(ctx, id, formattedValue, enum.SettingTypeIntegerList); err != nil {
		return err
	}

	co.eventEmitter.Emit(ctx, "SetInt64List", mrmsg.Data{"id": id})

	return nil
}

// SetBool - сохранение булева значения настройки с указанным идентификатором.
func (co *SettingsSetter) SetBool(ctx context.Context, id uint64, value bool) error {
	formattedValue, err := co.formatter.FormatBool(value)
	if err != nil {
		return err
	}

	if err = co.setValue(ctx, id, formattedValue, enum.SettingTypeBoolean); err != nil {
		return err
	}

	co.eventEmitter.Emit(ctx, "SetBool", mrmsg.Data{"id": id})

	return nil
}

func (co *SettingsSetter) setValue(ctx context.Context, id uint64, value string, rowType enum.SettingType) error {
	row := entity.Setting{
		ID:    id,
		Type:  rowType,
		Value: value,
	}

	if err := co.storage.Update(ctx, row); err != nil {
		return co.errorWrapper.WrapErrorEntityFailed(err, entity.ModelNameSetting, mrmsg.Data{"id": id})
	}

	return nil
}
