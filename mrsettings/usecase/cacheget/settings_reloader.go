package cacheget

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrlib/casttype"
	"github.com/mondegor/go-sysmess/mrlog"

	"github.com/mondegor/go-components/mrsettings"
	"github.com/mondegor/go-components/mrsettings/dto"
	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum"
)

type (
	// SettingsReloader - компонент для периодического обращения к настройкам в БД для обновления кэша.
	SettingsReloader struct {
		parser       mrsettings.ValueParser
		storage      mrsettings.StorageLoader
		errorWrapper mrerr.UseCaseErrorWrapper
		logger       mrlog.Logger
		cache        *sharedCache
		lastUpdated  time.Time
	}

	// sharedCache - разделяемый кэш, где хранятся настройки, загруженные из хранилища данных.
	sharedCache struct {
		mu       sync.RWMutex
		settings map[uint64]dto.CachedSetting
	}
)

// NewSettingsReloader - создаёт объект SettingsReloader.
func NewSettingsReloader(
	parser mrsettings.ValueParser,
	storage mrsettings.StorageLoader,
	errorWrapper mrerr.UseCaseErrorWrapper,
	logger mrlog.Logger,
) *SettingsReloader {
	return &SettingsReloader{
		parser:       parser,
		storage:      storage,
		errorWrapper: mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrsettings.SettingsReloader"),
		logger:       logger,
		cache: &sharedCache{
			mu:       sync.RWMutex{},
			settings: make(map[uint64]dto.CachedSetting),
		},
	}
}

// Reload - работа для обновления кэша настроек из БД.
func (jb *SettingsReloader) Reload(ctx context.Context) error {
	jb.cache.mu.RLock()
	lastUpdated := jb.lastUpdated
	jb.cache.mu.RUnlock()

	items, err := jb.storage.Fetch(ctx, lastUpdated)
	if err != nil {
		return jb.errorWrapper.WrapErrorFailed(err)
	}

	// обновления не требуется
	if len(items) == 0 {
		return nil
	}

	settings := make(map[uint64]dto.CachedSetting, len(items))

	for _, item := range items {
		id, setting, err := jb.makeItem(item)
		if err != nil {
			// ошибка некритичная, поэтому она просто логируется
			jb.logger.Error(ctx, "SettingsReloader", "error", err)

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
	jb.cache.mu.Lock()
	jb.cache.settings = settings
	jb.lastUpdated = lastUpdated
	jb.cache.mu.Unlock()

	jb.logger.Info(ctx, fmt.Sprintf("Settings are reloaded: %d", len(settings)))

	return nil
}

func (jb *SettingsReloader) makeItem(item entity.Setting) (id uint64, setting dto.CachedSetting, err error) {
	valueString, err := jb.parser.ParseString(item.Value)
	if err != nil {
		return 0, dto.CachedSetting{}, err
	}

	setting = dto.CachedSetting{
		Name:        item.Name,
		Type:        item.Type,
		ValueString: valueString, // строковое значение присутствует для всех типов
	}

	switch item.Type {
	case enum.SettingTypeStringList:
		setting.ValueStringList, err = jb.parser.ParseStringList(item.Value)
	case enum.SettingTypeInteger:
		setting.ValueInt64, err = jb.parser.ParseInt64(item.Value)
	case enum.SettingTypeIntegerList:
		setting.ValueInt64List, err = jb.parser.ParseInt64List(item.Value)
	case enum.SettingTypeBoolean:
		var boolValue bool
		boolValue, err = jb.parser.ParseBool(item.Value)
		setting.ValueInt64 = casttype.BoolToNumber[int64](boolValue)
	default:
		err = mr.ErrInternalUnhandledDefaultCase.New()
	}

	if err != nil {
		return 0, dto.CachedSetting{}, err
	}

	return item.ID, setting, nil
}
