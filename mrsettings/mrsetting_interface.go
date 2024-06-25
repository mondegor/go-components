package mrsettings

import (
	"context"
	"time"

	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrsettings/entity"
)

type (
	// Getter - comment interface.
	Getter interface {
		Get(ctx context.Context, id mrtype.KeyInt32) (string, error)
		GetList(ctx context.Context, id mrtype.KeyInt32) ([]string, error)
		GetInt64(ctx context.Context, id mrtype.KeyInt32) (int64, error)
		GetInt64List(ctx context.Context, id mrtype.KeyInt32) ([]int64, error)
		GetBool(ctx context.Context, id mrtype.KeyInt32) (bool, error)
	}

	// DefaultValueGetter - comment interface.
	DefaultValueGetter interface {
		Get(ctx context.Context, id mrtype.KeyInt32, defaultVal string) string
		GetList(ctx context.Context, id mrtype.KeyInt32, defaultVal []string) []string
		GetInt64(ctx context.Context, id mrtype.KeyInt32, defaultVal int64) int64
		GetInt64List(ctx context.Context, id mrtype.KeyInt32, defaultVal []int64) []int64
		GetBool(ctx context.Context, id mrtype.KeyInt32, defaultVal bool) bool
	}

	// Setter - comment interface.
	Setter interface {
		Set(ctx context.Context, id mrtype.KeyInt32, value string) error
		SetList(ctx context.Context, id mrtype.KeyInt32, value []string) error
		SetInt64(ctx context.Context, id mrtype.KeyInt32, value int64) error
		SetInt64List(ctx context.Context, id mrtype.KeyInt32, value []int64) error
		SetBool(ctx context.Context, id mrtype.KeyInt32, value bool) error
	}

	// Loader - comment interface.
	Loader interface {
		Reload(ctx context.Context) (count uint64, err error)
	}

	// ValueParser - comment interface.
	ValueParser interface {
		ParseString(value string) (string, error)
		ParseStringList(value string) ([]string, error)
		ParseInt64(value string) (int64, error)
		ParseInt64List(value string) ([]int64, error)
		ParseBool(value string) (bool, error)
	}

	// ValueFormatter - comment interface.
	ValueFormatter interface {
		FormatString(value string) (string, error)
		FormatStringList(values []string) (string, error)
		FormatInt64(value int64) (string, error)
		FormatInt64List(values []int64) (string, error)
		FormatBool(value bool) (string, error)
	}

	// ValueValidator - comment interface.
	ValueValidator interface {
		MatchString(s string) bool
		String() string
	}

	// StorageLoader - comment interface.
	StorageLoader interface {
		Fetch(ctx context.Context, lastUpdated time.Time) ([]entity.Setting, error)
	}

	// Storage - comment interface.
	Storage interface {
		FetchOne(ctx context.Context, id mrtype.KeyInt32) (entity.Setting, error)
		Update(ctx context.Context, row entity.Setting) error
	}
)
