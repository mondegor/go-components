package set

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrevent"

	"github.com/mondegor/go-components/mrsettings"
	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum"
)

type (
	// SettingsSetter - компонент для сохранения настроек в хранилище данных.
	SettingsSetter struct {
		txManager    mrstorage.DBTxManager
		formatter    mrsettings.ValueFormatter
		storage      mrsettings.Storage
		storageLog   mrsettings.StorageLog
		eventEmitter mrevent.Emitter
		errorWrapper mrerr.UseCaseErrorWrapper
	}
)

// New - создаёт объект SettingsSetter.
func New(
	txManager mrstorage.DBTxManager,
	formatter mrsettings.ValueFormatter,
	storage mrsettings.Storage,
	storageLog mrsettings.StorageLog,
	eventEmitter mrevent.Emitter,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *SettingsSetter {
	return &SettingsSetter{
		txManager:    txManager,
		formatter:    formatter,
		storage:      storage,
		storageLog:   storageLog,
		eventEmitter: mrevent.NewSourceEmitter(eventEmitter, entity.ModelNameSetting+".Setter"),
		errorWrapper: mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameSetting+".Setter"),
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

	co.eventEmitter.Emit(ctx, "Set", mrargs.Group{"id": id})

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

	co.eventEmitter.Emit(ctx, "SetList", mrargs.Group{"id": id})

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

	co.eventEmitter.Emit(ctx, "SetInt64", mrargs.Group{"id": id})

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

	co.eventEmitter.Emit(ctx, "SetInt64List", mrargs.Group{"id": id})

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

	co.eventEmitter.Emit(ctx, "SetBool", mrargs.Group{"id": id})

	return nil
}

func (co *SettingsSetter) setValue(ctx context.Context, id uint64, value string, rowType enum.SettingType) error {
	row := entity.Setting{
		ID:    id,
		Type:  rowType,
		Value: value,
	}

	return co.txManager.Do(ctx, func(ctx context.Context) error {
		if err := co.storage.Update(ctx, row); err != nil {
			return co.errorWrapper.WrapErrorFailed(err, "itemId", id)
		}

		if err := co.storageLog.Insert(ctx, id, value); err != nil {
			return co.errorWrapper.WrapErrorFailed(err, "itemId", id)
		}

		return nil
	})
}
