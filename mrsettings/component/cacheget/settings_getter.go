package cacheget

import (
	"context"

	"github.com/mondegor/go-webcore/mrcore"

	"github.com/mondegor/go-components/mrsettings"
	"github.com/mondegor/go-components/mrsettings/enum"
)

const (
	settingsMapName = "settings.map"
)

type (
	// SettingsGetter - компонент для удобного обращения к настройкам, которые хранятся в хранилище данных.
	// Использует внутренний кэш для уменьшения нагрузки на хранилище данных.
	SettingsGetter struct {
		cache   *SharedCache
		storage mrsettings.StorageLoader
	}
)

// NewSettingsGetter - создаёт объект SettingsGetter.
func NewSettingsGetter(cache *SharedCache, storage mrsettings.StorageLoader) *SettingsGetter {
	return &SettingsGetter{
		cache:   cache,
		storage: storage,
	}
}

// Get - возвращает строковое значение настройки с указанным идентификатором.
func (co *SettingsGetter) Get(_ context.Context, id uint64) (string, error) {
	co.cache.mu.RLock()
	value, ok := co.cache.settings[id]
	co.cache.mu.RUnlock()

	if ok {
		return value.ValueString, nil
	}

	return "", mrcore.ErrInternalKeyNotFoundInSource.New(id, settingsMapName)
}

// GetList - возвращает список строковых значений настройки с указанным идентификатором.
func (co *SettingsGetter) GetList(_ context.Context, id uint64) ([]string, error) {
	co.cache.mu.RLock()
	value, ok := co.cache.settings[id]
	co.cache.mu.RUnlock()

	if ok && value.Type == enum.SettingTypeIntegerList {
		return value.ValueStringList, nil
	}

	return nil, mrcore.ErrInternalKeyNotFoundInSource.New(id, settingsMapName)
}

// GetInt64 - возвращает целое знаковое значение настройки с указанным идентификатором.
func (co *SettingsGetter) GetInt64(_ context.Context, id uint64) (int64, error) {
	co.cache.mu.RLock()
	value, ok := co.cache.settings[id]
	co.cache.mu.RUnlock()

	if ok && value.Type == enum.SettingTypeInteger {
		return value.ValueInt64, nil
	}

	return 0, mrcore.ErrInternalKeyNotFoundInSource.New(id, settingsMapName)
}

// GetInt64List - возвращает список целых знаковых значений настройки с указанным идентификатором.
func (co *SettingsGetter) GetInt64List(_ context.Context, id uint64) ([]int64, error) {
	co.cache.mu.RLock()
	value, ok := co.cache.settings[id]
	co.cache.mu.RUnlock()

	if ok && value.Type == enum.SettingTypeIntegerList {
		return value.ValueInt64List, nil
	}

	return nil, mrcore.ErrInternalKeyNotFoundInSource.New(id, settingsMapName)
}

// GetBool - возвращает булево значение настройки с указанным идентификатором.
func (co *SettingsGetter) GetBool(_ context.Context, id uint64) (bool, error) {
	co.cache.mu.RLock()
	value, ok := co.cache.settings[id]
	co.cache.mu.RUnlock()

	if ok && value.Type == enum.SettingTypeBoolean {
		return value.ValueInt64 > 0, nil
	}

	return false, mrcore.ErrInternalKeyNotFoundInSource.New(id, settingsMapName)
}
