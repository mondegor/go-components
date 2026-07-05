package parse

import (
	"strconv"
	"strings"

	"github.com/mondegor/go-sysmess/errors"
)

const (
	defaultListItemSeparator = ","
)

type (
	// FieldParser - объект для преобразования данных поступающих
	// из хранилища данных в формат для внешнего использования.
	FieldParser struct {
		itemSeparator string
	}
)

// New - создаёт объект FieldParser.
func New(opts ...Option) *FieldParser {
	o := options{
		parser: &FieldParser{},
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.parser.itemSeparator == "" {
		o.parser.itemSeparator = defaultListItemSeparator
	}

	return o.parser
}

// ParseString - возвращает само значение value, т.к. оно уже строковое.
func (p *FieldParser) ParseString(value string) (string, error) {
	return value, nil
}

// ParseStringList - разделяет разделителем строку и возвращает список строк.
func (p *FieldParser) ParseStringList(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}

	return strings.Split(value, p.itemSeparator), nil
}

// ParseInt64 - возвращает целое знаковое число.
func (p *FieldParser) ParseInt64(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}

	parsedValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, errors.WrapInternalError(err, "parsing Int64 value failed")
	}

	return parsedValue, nil
}

// ParseInt64List - разделяет разделителем строку и список целых знаковых чисел.
func (p *FieldParser) ParseInt64List(value string) ([]int64, error) {
	if value == "" {
		return nil, nil
	}

	values := strings.Split(value, p.itemSeparator)
	parsedValues := make([]int64, len(values))

	for i := range values {
		parsedValue, err := strconv.ParseInt(values[i], 10, 64)
		if err != nil {
			return nil, errors.WrapInternalError(err, "parsing Int64List value failed")
		}

		parsedValues[i] = parsedValue
	}

	return parsedValues, nil
}

// ParseBool - возвращает булево значение.
func (p *FieldParser) ParseBool(value string) (bool, error) {
	if value == "" {
		return false, nil
	}

	parsedValue, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.WrapInternalError(err, "parsing Bool value failed")
	}

	return parsedValue, nil
}
