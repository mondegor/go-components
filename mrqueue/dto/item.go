package dto

import "time"

type (
	// Item - элемент очереди.
	Item struct {
		ID            uint64
		ReadyDelayed  time.Duration
		RetryAttempts int16
	}
)
