package contactaddress

import (
	"strings"

	"github.com/mondegor/go-components/mrauth/enum/addresstype"
)

// NewEmail - создаёт объект ContactAddress с типом Email.
func NewEmail(value string) ContactAddress {
	return ContactAddress{
		kind: addresstype.Email,
		// original: value,
		value: strings.ToLower(value),
	}
}

// ParseEmail - преобразует строковое представление емаила и возвращает его в виде структуры,
// или, если преобразование не удалось, возвращает ошибку.
func ParseEmail(value string) (ContactAddress, error) {
	if len(value) < minLength || len(value) > maxLength {
		return ContactAddress{}, ErrEmailIsInvalid
	}

	return parseEmail(value)
}

func parseEmail(value string) (ContactAddress, error) {
	if !ValidateEmail(value) {
		return ContactAddress{}, ErrEmailIsInvalid
	}

	return ContactAddress{
		kind: addresstype.Email,
		// original: value,
		value: strings.ToLower(value),
	}, nil
}
