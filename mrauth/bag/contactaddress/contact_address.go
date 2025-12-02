package contactaddress

import (
	"strconv"

	"github.com/mondegor/go-components/mrauth/enum/addresstype"
)

type (
	// ContactAddress - comment struct.
	ContactAddress struct {
		Type     addresstype.Enum
		Original string
		Value    string
	}
)

// NewEmail - создаёт объект ContactAddress с типом емаил.
func NewEmail(value string) ContactAddress {
	return ContactAddress{
		Type:     addresstype.Email,
		Original: value,
		Value:    value,
	}
}

// NewPhone - создаёт объект ContactAddress с типом телефон.
func NewPhone(value string) ContactAddress {
	return ContactAddress{
		Type:     addresstype.Phone,
		Original: value,
		Value:    value,
	}
}

// NewDigitPhone - создаёт объект ContactAddress с типом телефон.
func NewDigitPhone(value uint64) ContactAddress {
	phoneString := strconv.FormatUint(value, 10)

	return ContactAddress{
		Type:     addresstype.Phone,
		Original: phoneString,
		Value:    phoneString,
	}
}
