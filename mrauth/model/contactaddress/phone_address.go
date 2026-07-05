package contactaddress

import (
	"strconv"
	"strings"

	"github.com/mondegor/go-components/mrauth/enum/addresstype"
)

const (
	minPhoneLength = 10
	maxPhoneLength = 16
)

// NewPhone - создаёт объект ContactAddress с типом Phone.
func NewPhone(value string) ContactAddress {
	return ContactAddress{
		kind: addresstype.Phone,
		// original: value,
		value: strings.ToLower(value),
	}
}

// NewDigitPhone - создаёт объект ContactAddress с типом Phone.
func NewDigitPhone(value uint64) ContactAddress {
	phoneString := strconv.FormatUint(value, 10)

	return ContactAddress{
		kind: addresstype.Phone,
		// original: phoneString,
		value: strings.ToLower(phoneString),
	}
}

// ParsePhone - преобразует строковое представление телефона и возвращает его в виде структуры,
// или, если преобразование не удалось, возвращает ошибку.
func ParsePhone(value string) (ContactAddress, error) {
	if len(value) < minPhoneLength || len(value) > maxLength {
		return ContactAddress{}, ErrPhoneIsInvalid
	}

	return parsePhone(value)
}

func parsePhone(value string) (ContactAddress, error) {
	if !ValidatePhone(value) {
		return ContactAddress{}, ErrPhoneIsInvalid
	}

	phoneString := strings.Map(
		func(r rune) rune {
			if r > '9' || r < '0' {
				return -1
			}

			return r
		},
		value,
	)

	if len(phoneString) < minPhoneLength || len(phoneString) > maxPhoneLength {
		return ContactAddress{}, ErrPhoneIsInvalid
	}

	phoneString = correctPhoneNumber(phoneString)

	phoneDigit, err := strconv.ParseUint(phoneString, 10, 64)
	if err != nil {
		return ContactAddress{}, ErrPhoneIsInvalid
	}

	return ContactAddress{
		kind: addresstype.Phone,
		// original:   value,
		value:      phoneString,
		digitValue: phoneDigit,
	}, nil
}

func correctPhoneNumber(value string) string {
	firstChar := value[0]

	// correct russian phone number: 8 -> 7
	if len(value) == 11 && firstChar == '8' {
		return "7" + value[1:]
	}

	// // correct russian phone number: add 7
	// if len(value) == 10 && (firstChar == '9' || firstChar == '8' || firstChar == '4' || firstChar == '3') {
	// 	return "7" + value
	// }

	return value
}
