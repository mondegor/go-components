package setter

import (
	"context"

	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrsettings"
)

type (
	// Component - компонент для сохранения настроек в хранилище данных.
	Component struct {
		formatter    mrsettings.ValueFormatter
		storage      mrsettings.Storage
		eventEmitter mrsender.EventEmitter
		errorWrapper mrcore.UsecaseErrorWrapper
	}
)

// New - создаёт объект Component.
func New(
	formatter mrsettings.ValueFormatter,
	storage mrsettings.Storage,
	eventEmitter mrsender.EventEmitter,
	errorWrapper mrcore.UsecaseErrorWrapper,
) *Component {
	return &Component{
		formatter:    formatter,
		storage:      storage,
		eventEmitter: eventEmitter,
		errorWrapper: errorWrapper,
	}
}

// Set - comment method.
func (co *Component) Set(ctx context.Context, id mrtype.KeyInt32, value string) error {
	formattedValue, err := co.formatter.FormatString(value)
	if err != nil {
		return err
	}

	if err = co.setValue(ctx, id, formattedValue, mrsettings.SettingTypeString); err != nil {
		return err
	}

	co.emitEvent(ctx, "Set", mrmsg.Data{"id": id})

	return nil
}

// SetList - comment method.
func (co *Component) SetList(ctx context.Context, id mrtype.KeyInt32, value []string) error {
	formattedValue, err := co.formatter.FormatStringList(value)
	if err != nil {
		return err
	}

	if err = co.setValue(ctx, id, formattedValue, mrsettings.SettingTypeIntegerList); err != nil {
		return err
	}

	co.emitEvent(ctx, "SetList", mrmsg.Data{"id": id})

	return nil
}

// SetInt64 - comment method.
func (co *Component) SetInt64(ctx context.Context, id mrtype.KeyInt32, value int64) error {
	formattedValue, err := co.formatter.FormatInt64(value)
	if err != nil {
		return err
	}

	if err = co.setValue(ctx, id, formattedValue, mrsettings.SettingTypeInteger); err != nil {
		return err
	}

	co.emitEvent(ctx, "SetInt64", mrmsg.Data{"id": id})

	return nil
}

// SetInt64List - comment method.
func (co *Component) SetInt64List(ctx context.Context, id mrtype.KeyInt32, value []int64) error {
	formattedValue, err := co.formatter.FormatInt64List(value)
	if err != nil {
		return err
	}

	if err = co.setValue(ctx, id, formattedValue, mrsettings.SettingTypeIntegerList); err != nil {
		return err
	}

	co.emitEvent(ctx, "SetInt64List", mrmsg.Data{"id": id})

	return nil
}

// SetBool - comment method.
func (co *Component) SetBool(ctx context.Context, id mrtype.KeyInt32, value bool) error {
	formattedValue, err := co.formatter.FormatBool(value)
	if err != nil {
		return err
	}

	if err = co.setValue(ctx, id, formattedValue, mrsettings.SettingTypeBoolean); err != nil {
		return err
	}

	co.emitEvent(ctx, "SetBool", mrmsg.Data{"id": id})

	return nil
}

func (co *Component) setValue(ctx context.Context, id mrtype.KeyInt32, value string, rowType mrsettings.SettingType) error {
	row := mrsettings.EntitySetting{
		ID:    id,
		Type:  rowType,
		Value: value,
	}

	if err := co.storage.Update(ctx, row); err != nil {
		return co.errorWrapper.WrapErrorEntityFailed(err, mrsettings.ModelNameEntitySetting, mrmsg.Data{"id": id})
	}

	return nil
}

func (co *Component) emitEvent(ctx context.Context, eventName string, object mrmsg.Data) {
	co.eventEmitter.EmitWithSource(
		ctx,
		"mrsettings.setter."+eventName,
		mrsettings.ModelNameEntitySetting,
		object,
	)
}
