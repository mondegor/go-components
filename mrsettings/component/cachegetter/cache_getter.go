package cachegetter

import (
	"context"
	"sync"
	"time"

	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrlog"
	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrsettings"
	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum"
)

const (
	settingsMapName = "settings.map"
)

type (
	// Component - компонент для удобного обращения к настройкам, которые хранятся
	// в хранилище данных. Использует внутренний кэш для уменьшения нагрузки на хранилище данных.
	Component struct {
		parser       mrsettings.ValueParser
		storage      mrsettings.StorageLoader
		errorWrapper mrcore.UsecaseErrorWrapper
		reloadMu     sync.Mutex
		lastUpdated  time.Time
		settingsMu   sync.RWMutex
		settings     map[mrtype.KeyInt32]entity.CachedSetting
	}
)

// New - создаёт объект Component.
func New(parser mrsettings.ValueParser, storage mrsettings.StorageLoader, errorWrapper mrcore.UsecaseErrorWrapper) *Component {
	return &Component{
		parser:       parser,
		storage:      storage,
		errorWrapper: errorWrapper,
		settings:     make(map[mrtype.KeyInt32]entity.CachedSetting, 0),
	}
}

// Get - comment method.
func (co *Component) Get(_ context.Context, id mrtype.KeyInt32) (string, error) {
	co.settingsMu.RLock()
	value, ok := co.settings[id]
	co.settingsMu.RUnlock()

	if ok {
		return value.ValueString, nil
	}

	return "", mrcore.ErrInternalKeyNotFoundInSource.New(id, settingsMapName)
}

// GetList - comment method.
func (co *Component) GetList(_ context.Context, id mrtype.KeyInt32) ([]string, error) {
	co.settingsMu.RLock()
	value, ok := co.settings[id]
	co.settingsMu.RUnlock()

	if ok && value.Type == enum.SettingTypeIntegerList {
		return value.ValueStringList, nil
	}

	return nil, mrcore.ErrInternalKeyNotFoundInSource.New(id, settingsMapName)
}

// GetInt64 - comment method.
func (co *Component) GetInt64(_ context.Context, id mrtype.KeyInt32) (int64, error) {
	co.settingsMu.RLock()
	value, ok := co.settings[id]
	co.settingsMu.RUnlock()

	if ok && value.Type == enum.SettingTypeInteger {
		return value.ValueInt64, nil
	}

	return 0, mrcore.ErrInternalKeyNotFoundInSource.New(id, settingsMapName)
}

// GetInt64List - comment method.
func (co *Component) GetInt64List(_ context.Context, id mrtype.KeyInt32) ([]int64, error) {
	co.settingsMu.RLock()
	value, ok := co.settings[id]
	co.settingsMu.RUnlock()

	if ok && value.Type == enum.SettingTypeIntegerList {
		return value.ValueInt64List, nil
	}

	return nil, mrcore.ErrInternalKeyNotFoundInSource.New(id, settingsMapName)
}

// GetBool - comment method.
func (co *Component) GetBool(_ context.Context, id mrtype.KeyInt32) (bool, error) {
	co.settingsMu.RLock()
	value, ok := co.settings[id]
	co.settingsMu.RUnlock()

	if ok && value.Type == enum.SettingTypeBoolean {
		return value.ValueInt64 > 0, nil
	}

	return false, mrcore.ErrInternalKeyNotFoundInSource.New(id, settingsMapName)
}

// Reload - comment method.
func (co *Component) Reload(ctx context.Context) (count uint64, err error) {
	if !co.reloadMu.TryLock() {
		return 0, nil
	}
	defer co.reloadMu.Unlock()

	items, err := co.storage.Fetch(ctx, co.lastUpdated)
	if err != nil {
		return 0, co.errorWrapper.WrapErrorFailed(err, entity.ModelNameSetting)
	}

	// обновления не требуется
	if len(items) == 0 {
		return 0, nil
	}

	settings := make([]entity.CachedSettingWithID, 0, len(items))

	for _, item := range items {
		setting, err := co.makeItem(item)
		if err != nil {
			mrlog.Ctx(ctx).Error().Err(err).Send()

			continue
		}

		settings = append(settings, setting)

		if item.UpdatedAt.After(co.lastUpdated) {
			co.lastUpdated = item.UpdatedAt
		}
	}

	// все обновления оказались с ошибками
	if len(settings) == 0 {
		return 0, nil
	}

	// обновление заранее подготовленных настроек
	co.settingsMu.Lock()
	for i := range settings {
		co.settings[settings[i].ID] = settings[i].CachedSetting
	}
	co.settingsMu.Unlock()

	return uint64(len(settings)), nil
}

func (co *Component) makeItem(item entity.Setting) (setting entity.CachedSettingWithID, err error) {
	valueString, err := co.parser.ParseString(item.Value)
	if err != nil {
		return entity.CachedSettingWithID{}, err
	}

	setting = entity.CachedSettingWithID{
		ID: item.ID,
		CachedSetting: entity.CachedSetting{
			Name:        item.Name,
			Type:        item.Type,
			ValueString: valueString, // строковое значение присутствует для всех типов
		},
	}

	switch item.Type {
	case enum.SettingTypeStringList:
		setting.ValueStringList, err = co.parser.ParseStringList(item.Value)
	case enum.SettingTypeInteger:
		setting.ValueInt64, err = co.parser.ParseInt64(item.Value)
	case enum.SettingTypeIntegerList:
		setting.ValueInt64List, err = co.parser.ParseInt64List(item.Value)
	case enum.SettingTypeBoolean:
		var boolValue bool
		boolValue, err = co.parser.ParseBool(item.Value)
		setting.ValueInt64 = mrtype.BoolToInt64(boolValue)
	}

	if err != nil {
		return entity.CachedSettingWithID{}, err
	}

	return setting, nil
}
