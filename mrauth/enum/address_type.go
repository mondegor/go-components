package enum

// Типы адресов для связи с пользователем.
const (
	AddressTypeEmail AddressType = iota + 1 // электронный адрес
	AddressTypePhone                        // номер телефона
)

type (
	// AddressType - тип уникальной строки используемой в качестве логина.
	AddressType uint8
)
