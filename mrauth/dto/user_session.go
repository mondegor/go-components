package dto

import (
	"time"
)

type (
	// UserSession - открытая сессия пользователя для выдачи в публичном API.
	UserSession struct {
		SessionID  uint32
		AppName    string
		DeviceName string
		LastIP     string
		Location   string
		CreatedAt  time.Time // время создания сессии
		UpdatedAt  time.Time // время последней активности сессии
		IsCurrent  bool
	}
)
