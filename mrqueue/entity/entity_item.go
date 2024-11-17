package entity

import "time"

const (
	ModelNameItem = "mrqueue.Item" // ModelNameItem - название сущности
)

type (
	// Item - элемент очереди.
	Item struct {
		ID            uint64
		ReadyDelayed  time.Duration
		RetryAttempts uint32
	}

	// ItemWithError - элемент очереди с причиной ошибки.
	ItemWithError struct {
		ID  uint64
		Err error
	}
)
