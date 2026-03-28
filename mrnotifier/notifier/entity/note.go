package entity

const (
	// ModelNameNotice - название сущности.
	ModelNameNotice = "mrnotifier.Notice"
)

type (
	// Note - несформированное уведомление для получателя.
	Note struct {
		ID   uint64
		Key  string
		Data map[string]string
	}
)

// MessageID - comment method.
func (e Note) MessageID() uint64 {
	return e.ID
}
