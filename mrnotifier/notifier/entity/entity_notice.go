package entity

const (
	// ModelNameNotice - название сущности.
	ModelNameNotice = "mrnotifier.Notice"
)

type (
	// Notice - уведомление для получателя.
	Notice struct {
		ID   uint64
		Key  string
		Data map[string]string
	}
)
