package dto

type (
	// Realm - сообщение для получателя.
	Realm struct {
		Name      string
		UserKinds []UserKind
	}

	// UserKind - сообщение для получателя.
	UserKind struct {
		Kind  string
		Roles []string
	}

	// CreateUserRealm - сообщение для получателя.
	CreateUserRealm struct {
		Name     string
		UserKind string
	}
)
