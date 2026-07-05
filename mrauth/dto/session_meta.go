package dto

import (
	"github.com/mondegor/go-core/mrtype"
)

type (
	// SessionMeta - метаданные клиента, фиксируемые при открытии сессии.
	// UserAgent и ClientIP - недоверенный ввод, контролируемый клиентом.
	SessionMeta struct {
		UserAgent string
		ClientIP  mrtype.DetailedIP
	}
)
