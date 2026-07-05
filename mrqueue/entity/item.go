package entity

type (
	// CrashedItem - сломанный элемент очереди с причиной ошибки.
	CrashedItem struct {
		ID    uint64
		Cause string
	}
)
