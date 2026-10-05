package dto

import (
	"time"

	"github.com/mondegor/go-components/mrauth/enum/securityevent"
)

type (
	// SecurityLogItem - событие журнала безопасности пользователя для выдачи в публичном API.
	// Подробности события (OldValue, NewValue, Factor, Remaining) заданы только у относящихся
	// к ним типов событий.
	SecurityLogItem struct {
		RecordID   int64
		EventType  securityevent.Enum
		AppName    string
		DeviceName string
		IP         string
		Location   string
		OldValue   string
		NewValue   string
		Factor     string
		Remaining  *int
		CreatedAt  time.Time
	}
)
