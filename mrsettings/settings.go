package mrsettings

import (
	"context"
)

type (
	// Getter - интерфейс получения значения настройки по указанному SettingID.
	Getter interface {
		Get(ctx context.Context, id uint64) (string, error)
		GetList(ctx context.Context, id uint64) ([]string, error)
		GetInt64(ctx context.Context, id uint64) (int64, error)
		GetInt64List(ctx context.Context, id uint64) ([]int64, error)
		GetBool(ctx context.Context, id uint64) (bool, error)
	}

	// MustGetter - интерфейс получения значения настройки по указанному SettingID.
	// Если значение не найдено или случилась ошибка, то будет возвращено значение по умолчанию.
	MustGetter interface {
		Get(ctx context.Context, id uint64, defValue string) string
		GetList(ctx context.Context, id uint64, defValue []string) []string
		GetInt64(ctx context.Context, id uint64, defValue int64) int64
		GetInt64List(ctx context.Context, id uint64, defValue []int64) []int64
		GetBool(ctx context.Context, id uint64, defValue bool) bool
	}

	// Setter - интерфейс сохранения значения настройки по указанному SettingID.
	Setter interface {
		Set(ctx context.Context, id uint64, value string) error
		SetList(ctx context.Context, id uint64, value []string) error
		SetInt64(ctx context.Context, id uint64, value int64) error
		SetInt64List(ctx context.Context, id uint64, value []int64) error
		SetBool(ctx context.Context, id uint64, value bool) error
	}
)
