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

	// CreateRealmUser - сообщение для получателя.
	CreateRealmUser struct {
		Name     string
		UserKind string
	}
)
