package entity

const (
	// ModelNameSession - название сущности.
	ModelNameSession = "mrauth.Session"
)

type (
	// Session - сообщение для получателя.
	Session struct {
		Hash       string
		AppName    string
		DeviceName string
		LastIP     string
		Location   string
		IsCurrent  bool
	}
)
