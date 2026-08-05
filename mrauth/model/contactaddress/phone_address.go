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

// NewPhone - создаёт объект ContactAddress с типом Phone из значения, уже прошедшего
// проверку (ValidatePhone): номер нормализуется, как при разборе.
func NewPhone(value string) ContactAddress {
	address, err := makePhone(value)
	if err != nil {
		return ContactAddress{}
	}

	return address
}

// NewDigitPhone - создаёт объект ContactAddress с типом Phone.
func NewDigitPhone(value uint64) ContactAddress {
	phoneStr := strconv.FormatUint(value, 10)

	return ContactAddress{
		kind: addresstype.Phone,
		// original: phoneStr,
		value:      phoneStr,
		digitValue: value,
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
	if !regexpPhone.MatchString(value) {
		return ContactAddress{}, ErrPhoneIsInvalid
	}

	return makePhone(value)
}

func makePhone(value string) (ContactAddress, error) {
	value = correctPhoneNumber(value)

	if len(value) < minPhoneLength || len(value) > maxPhoneLength {
		return ContactAddress{}, ErrPhoneIsInvalid
	}

	// 0 тоже отсеивается, потому что строка вида "0000000000" будет валидна и вернёт 0
	phoneDigit, err := strconv.ParseUint(value, 10, 64)
	if err != nil || phoneDigit == 0 {
		return ContactAddress{}, ErrPhoneIsInvalid
	}

	return ContactAddress{
		kind: addresstype.Phone,
		// original:   value,
		value:      value,
		digitValue: phoneDigit,
	}, nil
}

func correctPhoneNumber(value string) string {
	value = strings.Map(
		func(r rune) rune {
			if r > '9' || r < '0' {
				return -1
			}

			return r
		},
		value,
	)

	if len(value) == 0 {
		return ""
	}

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
