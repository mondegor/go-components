package mrsettings

import (
	"context"
	"time"

	"github.com/mondegor/go-components/mrsettings/entity"
)

type (
	// Getter - интерфейс получения значения настройки по-указанному ID.
	Getter interface {
		Get(ctx context.Context, id uint64) (string, error)
		GetList(ctx context.Context, id uint64) ([]string, error)
		GetInt64(ctx context.Context, id uint64) (int64, error)
		GetInt64List(ctx context.Context, id uint64) ([]int64, error)
		GetBool(ctx context.Context, id uint64) (bool, error)
	}

	// DefaultValueGetter - интерфейс получения значения настройки по-указанному ID.
	// Если значение не найдено или случилась ошибка, то будет возвращено значение по умолчанию.
	DefaultValueGetter interface {
		Get(ctx context.Context, id uint64, defaultVal string) string
		GetList(ctx context.Context, id uint64, defaultVal []string) []string
		GetInt64(ctx context.Context, id uint64, defaultVal int64) int64
		GetInt64List(ctx context.Context, id uint64, defaultVal []int64) []int64
		GetBool(ctx context.Context, id uint64, defaultVal bool) bool
	}

	// Setter - интерфейс сохранения значения настройки по-указанному ID.
	Setter interface {
		Set(ctx context.Context, id uint64, value string) error
		SetList(ctx context.Context, id uint64, value []string) error
		SetInt64(ctx context.Context, id uint64, value int64) error
		SetInt64List(ctx context.Context, id uint64, value []int64) error
		SetBool(ctx context.Context, id uint64, value bool) error
	}

	// Loader - интерфейс загрузки данных из хранилища в область памяти,
	// для оперативного доступа за значениями настроек.
	Loader interface {
		Reload(ctx context.Context) (count uint64, err error)
	}

	// ValueParser - парсер значения настройки полученного из хранилища,
	// с целью приведения к нужному типу данных.
	ValueParser interface {
		ParseString(value string) (string, error)
		ParseStringList(value string) ([]string, error)
		ParseInt64(value string) (int64, error)
		ParseInt64List(value string) ([]int64, error)
		ParseBool(value string) (bool, error)
	}

	// ValueFormatter - форматер значения настройки, который подготавливает
	// его к сохранению в хранилище данных. Если необходима валидация данных,
	// то она должна происходить до этапа форматирования.
	ValueFormatter interface {
		FormatString(value string) (string, error)
		FormatStringList(values []string) (string, error)
		FormatInt64(value int64) (string, error)
		FormatInt64List(values []int64) (string, error)
		FormatBool(value bool) (string, error)
	}

	// StorageLoader - выборка последних обновлённых данных в хранилище.
	StorageLoader interface {
		Fetch(ctx context.Context, lastUpdated time.Time) ([]entity.Setting, error)
	}

	// Storage - извлечение и сохранение значения настройки по-указанному ID.
	Storage interface {
		FetchOne(ctx context.Context, id uint64) (entity.Setting, error)
		Update(ctx context.Context, row entity.Setting) error
	}
)
