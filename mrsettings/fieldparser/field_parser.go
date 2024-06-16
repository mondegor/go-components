package fieldparser

import (
	"strconv"
	"strings"

	"github.com/mondegor/go-webcore/mrcore"
)

const (
	listItemSeparator = ","
)

type (
	// DBFieldParser - объект для преобразования данных поступающих
	// из хранилища данных в формат для внешнего использования.
	DBFieldParser struct {
		itemSeparator string
	}
)

// New - создаёт объект DBFieldParser.
func New(itemSeparator string) *DBFieldParser {
	if itemSeparator == "" {
		itemSeparator = listItemSeparator
	}

	return &DBFieldParser{
		itemSeparator: itemSeparator,
	}
}

// ParseString - comment method.
func (p *DBFieldParser) ParseString(value string) (string, error) {
	return value, nil
}

// ParseStringList - comment method.
func (p *DBFieldParser) ParseStringList(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}

	return strings.Split(value, p.itemSeparator), nil
}

// ParseInt64 - comment method.
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

// ParseInt64List - comment method.
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

// ParseBool - comment method.
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
