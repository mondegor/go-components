package fieldformatter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mondegor/go-webcore/mrcore"
)

const (
	valueMaxLen       = 65536
	listItemSeparator = ","
)

type (
	// DBFieldFormatter - объект для преобразования данных поступающих
	// из внешнего источника в формат для сохранения в хранилище данных.
	DBFieldFormatter struct {
		valueMaxLen   int
		itemSeparator string
	}

	// DBFieldFormatterOptions - опции для создания DBFieldFormatter.
	DBFieldFormatterOptions struct {
		ValueMaxLen       uint64 // optional
		ListItemSeparator string // optional
	}
)

// New - создаёт объект DBFieldFormatter.
func New(opts DBFieldFormatterOptions) *DBFieldFormatter {
	if opts.ValueMaxLen == 0 {
		opts.ValueMaxLen = valueMaxLen
	}

	if opts.ListItemSeparator == "" {
		opts.ListItemSeparator = listItemSeparator
	}

	return &DBFieldFormatter{
		valueMaxLen:   int(opts.ValueMaxLen),
		itemSeparator: opts.ListItemSeparator,
	}
}

// FormatString - comment method.
func (f *DBFieldFormatter) FormatString(value string) (string, error) {
	if len(value) > f.valueMaxLen {
		return "", mrcore.ErrInternalValueLenMax.New(len(value), f.valueMaxLen)
	}

	return value, nil
}

// FormatStringList - comment method.
func (f *DBFieldFormatter) FormatStringList(values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}

	valuesLen := (len(values) - 1) * len(f.itemSeparator) // separator's len
	for i := range values {
		valuesLen += len(values[i])
	}

	if valuesLen > f.valueMaxLen {
		return "", mrcore.ErrInternalValueLenMax.New(valuesLen, f.valueMaxLen)
	}

	var buf strings.Builder

	buf.Grow(valuesLen)

	for i := range values {
		if i > 0 {
			buf.WriteString(f.itemSeparator)
		}

		buf.WriteString(values[i])
	}

	return buf.String(), nil
}

// FormatInt64 - comment method.
func (f *DBFieldFormatter) FormatInt64(value int64) (string, error) {
	return strconv.FormatInt(value, 10), nil
}

// FormatInt64List - comment method.
func (f *DBFieldFormatter) FormatInt64List(values []int64) (string, error) {
	if len(values) == 0 {
		return "", nil
	}

	const maxInt64Digits = 19 // max number of digits in int64

	if len(values) > f.valueMaxLen/maxInt64Digits {
		return "", mrcore.ErrInternal.Wrap(
			fmt.Errorf(
				"number of digits cannot be more than %d; got: %d, maximum field length: %d",
				f.valueMaxLen/maxInt64Digits,
				len(values),
				f.valueMaxLen,
			),
		)
	}

	var buf strings.Builder

	buf.Grow(len(values)*2 + (len(values)-1)*len(f.itemSeparator)) // 2 digits per item + separator's len

	for i := range values {
		if i > 0 {
			buf.WriteString(f.itemSeparator)
		}

		buf.WriteString(strconv.FormatInt(values[i], 10))
	}

	return buf.String(), nil
}

// FormatBool - comment method.
func (f *DBFieldFormatter) FormatBool(value bool) (string, error) {
	return strconv.FormatBool(value), nil
}
