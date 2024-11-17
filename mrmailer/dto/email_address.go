package dto

type (
	// EmailAddress - емаил адрес.
	EmailAddress struct {
		Name  string `json:"name,omitempty"`
		Email string `json:"email"`
	}
)

// Empty - проверяет, что объект пустой.
func (e *EmailAddress) Empty() bool {
	return e.Email == ""
}

// String - возвращается емаил адрес в виде строки.
func (e *EmailAddress) String() string {
	if e.Name == "" {
		return e.Email
	}

	return e.Name + " <" + e.Email + ">"
}
