package entity

import (
	"time"

	"github.com/mondegor/go-components/mrsettings/enum/settingtype"
)

type (
	// Setting - элемент с метаинформацией настройки и её значением.
	Setting struct {
		ID          uint64
		Name        string
		Type        settingtype.Enum
		Value       string
		Description string    // only for fetch all
		UpdatedAt   time.Time // only for fetch all
	}
)
