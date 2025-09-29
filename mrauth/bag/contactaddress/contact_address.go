package contactaddress

import (
	"strconv"

	"github.com/mondegor/go-components/mrauth/enum"
)

type (
	// ContactAddress - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ContactAddress struct {
		Type     enum.AddressType
		Original string
		Value    string
	}
)

// NewEmail - создаёт объект ContactAddress с типом емаил.
func NewEmail(value string) ContactAddress {
	return ContactAddress{
		Type:     enum.AddressTypeEmail,
		Original: value,
		Value:    value,
	}
}

// NewPhone - создаёт объект ContactAddress с типом телефон.
func NewPhone(value string) ContactAddress {
	return ContactAddress{
		Type:     enum.AddressTypePhone,
		Original: value,
		Value:    value,
	}
}

// NewDigitPhone - создаёт объект ContactAddress с типом телефон.
func NewDigitPhone(value uint64) ContactAddress {
	phoneString := strconv.FormatUint(value, 10)

	return ContactAddress{
		Type:     enum.AddressTypePhone,
		Original: phoneString,
		Value:    phoneString,
	}
}
