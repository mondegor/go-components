package fieldparser

import (
	"strconv"
	"strings"

	"github.com/mondegor/go-webcore/mrcore"
)

const (
	defaultListItemSeparator = ","
)

type (
	// DBFieldParser - объект для преобразования данных поступающих
	// из хранилища данных в формат для внешнего использования.
	DBFieldParser struct {
		itemSeparator string
	}
)

// New - создаёт объект DBFieldParser.
func New(opts ...Option) *DBFieldParser {
	p := &DBFieldParser{
		itemSeparator: defaultListItemSeparator,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// ParseString - возвращает само значение value, т.к. оно уже строковое.
func (p *DBFieldParser) ParseString(value string) (string, error) {
	return value, nil
}

// ParseStringList - разделяет разделителем строку и возвращает список строк.
func (p *DBFieldParser) ParseStringList(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}

	return strings.Split(value, p.itemSeparator), nil
}

// ParseInt64 - возвращает целое знаковое число.
func (p *DBFieldParser) ParseInt64(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}

	parsedValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, mrcore.ErrInternal.Wrap(err)
	}

	return parsedValue, nil
}

// ParseInt64List - разделяет разделителем строку и список целых знаковых чисел.
func (p *DBFieldParser) ParseInt64List(value string) ([]int64, error) {
	if value == "" {
		return nil, nil
	}

	values := strings.Split(value, p.itemSeparator)
	parsedValues := make([]int64, len(values))

	for i := range values {
		parsedValue, err := strconv.ParseInt(values[i], 10, 64)
		if err != nil {
			return nil, mrcore.ErrInternal.Wrap(err)
		}

		parsedValues[i] = parsedValue
	}

	return parsedValues, nil
}

// ParseBool - возвращает булево значение.
func (p *DBFieldParser) ParseBool(value string) (bool, error) {
	if value == "" {
		return false, nil
	}

	parsedValue, err := strconv.ParseBool(value)
	if err != nil {
		return false, mrcore.ErrInternal.Wrap(err)
	}

	return parsedValue, nil
}
