package cacheget

import (
	"sync"
	"time"

	"github.com/mondegor/go-components/mrsettings/entity"
)

type (
	// SharedCache - разделяемый кэш, где хранятся настройки, загруженные из хранилища данных.
	SharedCache struct {
		mu          sync.RWMutex
		settings    map[uint64]entity.CachedSetting
		lastUpdated time.Time
	}
)

// NewSharedCache - создаёт объект SharedCache.
func NewSharedCache() *SharedCache {
	return &SharedCache{
		mu:       sync.RWMutex{},
		settings: make(map[uint64]entity.CachedSetting),
	}
}
