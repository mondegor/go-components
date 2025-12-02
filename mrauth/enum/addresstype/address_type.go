package addresstype

// Типы адресов для связи с пользователем.
const (
	Email Enum = iota + 1 // электронный адрес
	Phone                 // номер телефона
)

type (
	// Enum - тип уникальной строки используемой в качестве логина.
	Enum uint8
)
