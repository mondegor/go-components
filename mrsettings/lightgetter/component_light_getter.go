package lightgetter

import (
	"context"

	"github.com/mondegor/go-webcore/mrlog"
	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrsettings"
)

type (
	// Component - компонент для извлечения настроек,
	// которые хранятся в хранилище данных.
	Component struct {
		reader mrsettings.Getter
	}
)

// New - создаёт объект Component.
func New(reader mrsettings.Getter) *Component {
	return &Component{
		reader: reader,
	}
}

// Get - comment method.
func (co *Component) Get(ctx context.Context, id mrtype.KeyInt32, defaultVal string) string {
	value, err := co.reader.Get(ctx, id)
	if err != nil {
		mrlog.Ctx(ctx).Error().Err(err).Send()

		return defaultVal
	}

	return value
}

// GetList - comment method.
func (co *Component) GetList(ctx context.Context, id mrtype.KeyInt32, defaultVal []string) []string {
	value, err := co.reader.GetList(ctx, id)
	if err != nil {
		mrlog.Ctx(ctx).Error().Err(err).Send()

		return defaultVal
	}

	return value
}

// GetInt64 - comment method.
func (co *Component) GetInt64(ctx context.Context, id mrtype.KeyInt32, defaultVal int64) int64 {
	value, err := co.reader.GetInt64(ctx, id)
	if err != nil {
		mrlog.Ctx(ctx).Error().Err(err).Send()

		return defaultVal
	}

	return value
}

// GetInt64List - comment method.
func (co *Component) GetInt64List(ctx context.Context, id mrtype.KeyInt32, defaultVal []int64) []int64 {
	value, err := co.reader.GetInt64List(ctx, id)
	if err != nil {
		mrlog.Ctx(ctx).Error().Err(err).Send()

		return defaultVal
	}

	return value
}

// GetBool - comment method.
func (co *Component) GetBool(ctx context.Context, id mrtype.KeyInt32, defaultVal bool) bool {
	value, err := co.reader.GetBool(ctx, id)
	if err != nil {
		mrlog.Ctx(ctx).Error().Err(err).Send()

		return defaultVal
	}

	return value
}
