package entity

const (
	ModelNameNotice = "mrnotifier.Notice" // ModelNameNotice - название сущности
)

type (
	// Notice - уведомление для получателя.
	Notice struct {
		ID   uint64
		Key  string
		Data map[string]string
	}
)
