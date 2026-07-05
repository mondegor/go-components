package contactaddress

import (
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
)

const (
	maxLength = 64
	minLength = 7
)

type (
	// ContactAddress - контактный адрес.
	ContactAddress struct {
		kind addresstype.Enum
		// original   string
		value      string
		digitValue uint64
	}
)

// Is - сообщает о соответствии адреса указанному типу.
func (a ContactAddress) Is(t addresstype.Enum) bool {
	return a.kind == t
}

// Original - возвращает оригинальный адрес в виде строки.
// func (a ContactAddress) Original() string {
// 	return a.original
// }

// Value - возвращает отформатированный адрес в виде строки.
func (a ContactAddress) Value() string {
	return a.value
}

// DigitValue - возвращает адрес в виде целого числа (актуально только для номера телефона).
func (a ContactAddress) DigitValue() uint64 {
	return a.digitValue
}

// Parse - преобразует строковое представление адреса и возвращает его в виде структуры,
// или, если преобразование не удалось, возвращает ошибку.
func Parse(value string) (ContactAddress, error) {
	if len(value) < minLength || len(value) > maxLength {
		return ContactAddress{}, ErrAddressIsInvalid
	}

	if address, err := parseEmail(value); err == nil {
		return address, nil
	}

	if address, err := parsePhone(value); err == nil {
		return address, nil
	}

	return ContactAddress{}, ErrAddressIsInvalid
}
