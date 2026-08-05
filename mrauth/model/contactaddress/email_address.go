package contactaddress

import (
	"strings"

	"github.com/mondegor/go-components/mrauth/enum/addresstype"
)

// NewEmail - создаёт объект ContactAddress с типом Email из значения,
// уже прошедшего проверку (ValidateEmail): емаил нормализуется, как при разборе.
func NewEmail(value string) ContactAddress {
	return makeEmail(value)
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
	if !regexpEmail.MatchString(value) {
		return ContactAddress{}, ErrEmailIsInvalid
	}

	return makeEmail(value), nil
}

func makeEmail(value string) ContactAddress {
	return ContactAddress{
		kind: addresstype.Email,
		// original: value,
		value: strings.ToLower(value),
	}
}
