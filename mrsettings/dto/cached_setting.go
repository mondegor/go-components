package dto

import (
	"github.com/mondegor/go-components/mrsettings/enum/settingtype"
)

type (
	// CachedSetting - элемент настройки для хранения её в кэше.
	CachedSetting struct {
		Name            string
		Type            settingtype.Enum
		ValueString     string
		ValueInt64      int64
		ValueStringList []string
		ValueInt64List  []int64
	}
)
