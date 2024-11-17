package lightget

import (
	"context"

	"github.com/mondegor/go-webcore/mrlog"

	"github.com/mondegor/go-components/mrsettings"
)

type (
	// SettingsGetter - компонент для извлечения настроек,
	// которые хранятся в хранилище данных.
	SettingsGetter struct {
		reader mrsettings.Getter
	}
)

// New - создаёт объект SettingsGetter.
func New(reader mrsettings.Getter) *SettingsGetter {
	return &SettingsGetter{
		reader: reader,
	}
}

// Get - возвращает строковое значение настройки с указанным идентификатором.
func (co *SettingsGetter) Get(ctx context.Context, id uint64, defValue string) string {
	value, err := co.reader.Get(ctx, id)
	if err != nil {
		mrlog.Ctx(ctx).Error().Err(err).Send()

		return defValue
	}

	return value
}

// GetList - возвращает список строковых значений настройки с указанным идентификатором.
func (co *SettingsGetter) GetList(ctx context.Context, id uint64, defValue []string) []string {
	value, err := co.reader.GetList(ctx, id)
	if err != nil {
		mrlog.Ctx(ctx).Error().Err(err).Send()

		return defValue
	}

	return value
}

// GetInt64 - возвращает целое знаковое значение настройки с указанным идентификатором.
func (co *SettingsGetter) GetInt64(ctx context.Context, id uint64, defValue int64) int64 {
	value, err := co.reader.GetInt64(ctx, id)
	if err != nil {
		mrlog.Ctx(ctx).Error().Err(err).Send()

		return defValue
	}

	return value
}

// GetInt64List - возвращает список целых знаковых значений настройки с указанным идентификатором.
func (co *SettingsGetter) GetInt64List(ctx context.Context, id uint64, defValue []int64) []int64 {
	value, err := co.reader.GetInt64List(ctx, id)
	if err != nil {
		mrlog.Ctx(ctx).Error().Err(err).Send()

		return defValue
	}

	return value
}

// GetBool - возвращает булево значение настройки с указанным идентификатором.
func (co *SettingsGetter) GetBool(ctx context.Context, id uint64, defValue bool) bool {
	value, err := co.reader.GetBool(ctx, id)
	if err != nil {
		mrlog.Ctx(ctx).Error().Err(err).Send()

		return defValue
	}

	return value
}
