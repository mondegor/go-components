package entity

import (
	"github.com/mondegor/go-components/mrsettings/enum"
)

type (
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
		ID uint64
		CachedSetting
	}
)
