package entity

import (
	"time"

	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrsettings/enum"
)

const (
	ModelNameSetting = "mrsettings.Setting" // ModelNameSetting - название сущности
)

type (
	// Setting - элемент с метаинформацией настройки и её значением.
	Setting struct {
		ID          mrtype.KeyInt32
		Name        string
		Type        enum.SettingType
		Value       string
		Description string    // only for fetch all
		UpdatedAt   time.Time // only for fetch all
	}

	// CachedSetting - элемент настройки для хранения её в кэше.
	CachedSetting struct {
		Name            string
		Type            enum.SettingType
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
