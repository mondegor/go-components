package mrsettings

import (
	"time"

	"github.com/mondegor/go-webcore/mrtype"
)

const (
	ModelNameEntitySetting = "EntitySetting" // ModelNameEntitySetting - название сущности
)

type (
	// EntitySetting - элемент с метаинформацией настройки и её значением.
	EntitySetting struct {
		ID          mrtype.KeyInt32
		Name        string
		Type        SettingType
		Value       string
		Description string    // only for fetch all
		UpdatedAt   time.Time // only for fetch all
	}

	// CachedSetting - элемент настройки для хранения её в кэше.
	CachedSetting struct {
		Name            string
		Type            SettingType
		ValueString     string
		ValueInt64      int64
		ValueStringList []string
		ValueInt64List  []int64
	}

	// CachedSettingWithID - элемент настройки CachedSetting с полем ID.
	CachedSettingWithID struct {
		ID mrtype.KeyInt32
		CachedSetting
	}
)
