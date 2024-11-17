package cacheget

import (
	"context"

	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrlog"
	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrsettings"
	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum"
)

type (
	// ReloadSettingsJob - компонент для периодического обращения к настройкам в БД для обновления кэша.
	ReloadSettingsJob struct {
		cache        *SharedCache
		parser       mrsettings.ValueParser
		storage      mrsettings.StorageLoader
		errorWrapper mrcore.UseCaseErrorWrapper
	}
)

// NewReloadSettingsJob - создаёт объект ReloadSettingsJob.
func NewReloadSettingsJob(
	cache *SharedCache,
	parser mrsettings.ValueParser,
	storage mrsettings.StorageLoader,
	errorWrapper mrcore.UseCaseErrorWrapper,
) *ReloadSettingsJob {
	return &ReloadSettingsJob{
		cache:        cache,
		parser:       parser,
		storage:      storage,
		errorWrapper: errorWrapper,
	}
}

// Do - работа для обновления кэша настроек из БД.
func (jb *ReloadSettingsJob) Do(ctx context.Context) error {
	jb.cache.mu.RLock()
	lastUpdated := jb.cache.lastUpdated
	jb.cache.mu.RUnlock()

	items, err := jb.storage.Fetch(ctx, lastUpdated)
	if err != nil {
		return jb.errorWrapper.WrapErrorFailed(err, entity.ModelNameSetting)
	}

	// обновления не требуется
	if len(items) == 0 {
		return nil
	}

	logger := mrlog.Ctx(ctx)
	settings := make([]entity.CachedSettingWithID, 0, len(items))

	for _, item := range items {
		setting, err := jb.makeItem(item)
		if err != nil {
			// ошибка не критичная, поэтому она просто логируется
			logger.Error().Err(err).Send()

			continue
		}

		settings = append(settings, setting)

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
	for i := range settings {
		jb.cache.settings[settings[i].ID] = settings[i].CachedSetting
	}

	jb.cache.lastUpdated = lastUpdated
	jb.cache.mu.Unlock()

	if len(settings) > 0 {
		logger.Info().Msgf("Settings are reloaded: %d", len(settings))
	}

	return nil
}

func (jb *ReloadSettingsJob) makeItem(item entity.Setting) (setting entity.CachedSettingWithID, err error) {
	valueString, err := jb.parser.ParseString(item.Value)
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
		setting.ValueStringList, err = jb.parser.ParseStringList(item.Value)
	case enum.SettingTypeInteger:
		setting.ValueInt64, err = jb.parser.ParseInt64(item.Value)
	case enum.SettingTypeIntegerList:
		setting.ValueInt64List, err = jb.parser.ParseInt64List(item.Value)
	case enum.SettingTypeBoolean:
		var boolValue bool
		boolValue, err = jb.parser.ParseBool(item.Value)
		setting.ValueInt64 = mrtype.CastBoolToNumber[int64](boolValue)
	default:
		err = mrcore.ErrInternalUnhandledDefaultCase.New()
	}

	if err != nil {
		return entity.CachedSettingWithID{}, err
	}

	return setting, nil
}
