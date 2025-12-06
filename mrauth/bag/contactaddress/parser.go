package contactaddress

import (
	"strings"

	"github.com/mondegor/go-components/mrauth/enum/addresstype"
)

const (
	maxLength      = 64
	minEmailLength = 7
	minPhoneLength = 10
	maxPhoneLength = 16
)

// Parser - comments struct.
type (
	Parser struct{}
)

// NewParser - создаёт объект Parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse - comments method.
func (p *Parser) Parse(value string) (ContactAddress, error) {
	if len(value) < minEmailLength || len(value) > maxLength {
		return ContactAddress{}, ErrLoginIsInvalid.New()
	}

	if address, err := p.parseEmail(value); err == nil {
		return address, nil
	}

	if address, err := p.parsePhone(value); err == nil {
		return address, nil
	}

	return ContactAddress{}, ErrLoginIsInvalid.New()
}

// ParseEmail - comments method.
func (p *Parser) ParseEmail(value string) (ContactAddress, error) {
	if len(value) < minEmailLength || len(value) > maxLength {
		return ContactAddress{}, ErrEmailIsInvalid.New()
	}

	return p.parseEmail(value)
}

// ParsePhone - comments method.
func (p *Parser) ParsePhone(value string) (ContactAddress, error) {
	if len(value) < minPhoneLength || len(value) > maxLength {
		return ContactAddress{}, ErrPhoneIsInvalid.New()
	}

	return p.parseEmail(value)
}

func (p *Parser) parseEmail(value string) (ContactAddress, error) {
	if !ValidateEmail(value) {
		return ContactAddress{}, ErrEmailIsInvalid.New()
	}

	return ContactAddress{
		Type:     addresstype.Email,
		Original: value,
		Value:    strings.ToLower(value),
	}, nil
}

func (p *Parser) parsePhone(value string) (ContactAddress, error) {
	if !ValidatePhone(value) {
		return ContactAddress{}, ErrPhoneIsInvalid.New()
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
		return ContactAddress{}, ErrPhoneIsInvalid.New()
	}

	return ContactAddress{
		Type:     addresstype.Phone,
		Original: value,
		Value:    correctPhoneNumber(phoneString),
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
