package cacheget

import (
	"context"
	"sync"
	"time"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"

	"github.com/mondegor/go-components/mrsettings/dto"
	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum/settingtype"
	"github.com/mondegor/go-components/mrsettings/field"
)

const (
	settingsMapName = "settings.cacheget.map"
)

type (
	// SettingsGetter - компонент для удобного обращения к настройкам, которые хранятся в хранилище данных.
	// Использует внутренний кэш для уменьшения нагрузки на хранилище данных.
	// Имеет метод обращения к настройкам в БД для обновления кэша.
	SettingsGetter struct {
		parser       field.ValueParser
		storage      storageLoader
		errorWrapper errors.Wrapper
		logger       mrlog.Logger
		cache        *settingsCache
		lastUpdated  time.Time
	}

	// settingsCache - кэш, где хранятся настройки, загруженные из хранилища данных.
	settingsCache struct {
		mu       sync.RWMutex
		settings map[uint64]dto.CachedSetting
	}

	storageLoader interface {
		Fetch(ctx context.Context, lastUpdated time.Time) ([]entity.Setting, error)
	}
)

// NewSettingsGetter - создаёт объект SettingsGetter.
func NewSettingsGetter(
	parser field.ValueParser,
	storage storageLoader,
	logger mrlog.Logger,
) *SettingsGetter {
	return &SettingsGetter{
		parser:       parser,
		storage:      storage,
		errorWrapper: errors.NewServiceWrapper(),
		logger:       logger,
		cache: &settingsCache{
			mu:       sync.RWMutex{},
			settings: make(map[uint64]dto.CachedSetting),
		},
	}
}

// Get - возвращает строковое значение настройки с указанным идентификатором.
func (sv *SettingsGetter) Get(_ context.Context, id uint64) (string, error) {
	sv.cache.mu.RLock()
	value, ok := sv.cache.settings[id]
	sv.cache.mu.RUnlock()

	if ok {
		return value.ValueString, nil
	}

	return "", errors.ErrInternalKeyNotFoundInSource.New(
		"key", id,
		"source", settingsMapName,
	)
}

// GetList - возвращает список строковых значений настройки с указанным идентификатором.
func (sv *SettingsGetter) GetList(_ context.Context, id uint64) ([]string, error) {
	sv.cache.mu.RLock()
	value, ok := sv.cache.settings[id]
	sv.cache.mu.RUnlock()

	if ok && value.Type == settingtype.IntegerList {
		return value.ValueStringList, nil
	}

	return nil, errors.ErrInternalKeyNotFoundInSource.New(
		"key", id,
		"source", settingsMapName,
	)
}

// GetInt64 - возвращает целое знаковое значение настройки с указанным идентификатором.
func (sv *SettingsGetter) GetInt64(_ context.Context, id uint64) (int64, error) {
	sv.cache.mu.RLock()
	value, ok := sv.cache.settings[id]
	sv.cache.mu.RUnlock()

	if ok && value.Type == settingtype.Integer {
		return value.ValueInt64, nil
	}

	return 0, errors.ErrInternalKeyNotFoundInSource.New(
		"key", id,
		"source", settingsMapName,
	)
}

// GetInt64List - возвращает список целых знаковых значений настройки с указанным идентификатором.
func (sv *SettingsGetter) GetInt64List(_ context.Context, id uint64) ([]int64, error) {
	sv.cache.mu.RLock()
	value, ok := sv.cache.settings[id]
	sv.cache.mu.RUnlock()

	if ok && value.Type == settingtype.IntegerList {
		return value.ValueInt64List, nil
	}

	return nil, errors.ErrInternalKeyNotFoundInSource.New(
		"key", id,
		"source", settingsMapName,
	)
}

// GetBool - возвращает булево значение настройки с указанным идентификатором.
func (sv *SettingsGetter) GetBool(_ context.Context, id uint64) (bool, error) {
	sv.cache.mu.RLock()
	value, ok := sv.cache.settings[id]
	sv.cache.mu.RUnlock()

	if ok && value.Type == settingtype.Boolean {
		return value.ValueInt64 > 0, nil
	}

	return false, errors.ErrInternalKeyNotFoundInSource.New(
		"key", id,
		"source", settingsMapName,
	)
}
