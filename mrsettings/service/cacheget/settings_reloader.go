package cacheget

import (
	"context"
	"fmt"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/casttype"

	"github.com/mondegor/go-components/mrsettings/dto"
	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum/settingtype"
)

// Reload - работа для обновления кэша настроек из БД.
func (sv *SettingsGetter) Reload(ctx context.Context) error {
	sv.cache.mu.RLock()
	lastUpdated := sv.lastUpdated
	sv.cache.mu.RUnlock()

	items, err := sv.storage.Fetch(ctx, lastUpdated)
	if err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	// обновления не требуется
	if len(items) == 0 {
		return nil
	}

	settings := make(map[uint64]dto.CachedSetting, len(items))

	for _, item := range items {
		id, setting, err := sv.makeItem(item)
		if err != nil {
			// ошибка некритичная, поэтому она просто логируется
			sv.logger.Error(ctx, "SettingsReloader", "error", err)

			continue
		}

		settings[id] = setting

		if item.UpdatedAt.After(lastUpdated) {
			lastUpdated = item.UpdatedAt
		}
	}

	// все обновления оказались с ошибками
	if len(settings) == 0 {
		return nil
	}

	// обновление заранее подготовленных настроек
	sv.cache.mu.Lock()
	sv.cache.settings = settings
	sv.lastUpdated = lastUpdated
	sv.cache.mu.Unlock()

	sv.logger.Info(ctx, fmt.Sprintf("Settings are reloaded: %d", len(settings)))

	return nil
}

func (sv *SettingsGetter) makeItem(item entity.Setting) (id uint64, setting dto.CachedSetting, err error) {
	valueString, err := sv.parser.ParseString(item.Value)
	if err != nil {
		return 0, dto.CachedSetting{}, err
	}

	setting = dto.CachedSetting{
		Name:        item.Name,
		Type:        item.Type,
		ValueString: valueString, // строковое значение присутствует для всех типов
	}

	switch item.Type {
	case settingtype.StringList:
		setting.ValueStringList, err = sv.parser.ParseStringList(item.Value)
	case settingtype.Integer:
		setting.ValueInt64, err = sv.parser.ParseInt64(item.Value)
	case settingtype.IntegerList:
		setting.ValueInt64List, err = sv.parser.ParseInt64List(item.Value)
	case settingtype.Boolean:
		var boolValue bool
		boolValue, err = sv.parser.ParseBool(item.Value)
		setting.ValueInt64 = casttype.BoolToNumber[int64](boolValue)
	default:
		err = errors.NewInternalError("unhandled default case")
	}

	if err != nil {
		return 0, dto.CachedSetting{}, err
	}

	return item.ID, setting, nil
}
