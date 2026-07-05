package format

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mondegor/go-sysmess/errors"
)

const (
	defaultValueMaxLen       = 65536
	defaultListItemSeparator = ","
)

type (
	// FieldFormatter - объект для преобразования данных поступающих
	// из внешнего источника в формат для сохранения в хранилище данных.
	FieldFormatter struct {
		valueMaxLen   int
		itemSeparator string
	}
)

var errInternalValueLenMax = errors.NewInternalProto("value has length greater then max characters")

// New - создаёт объект FieldFormatter.
func New(opts ...Option) *FieldFormatter {
	o := options{
		formatter: &FieldFormatter{},
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.formatter.valueMaxLen < 1 {
		o.formatter.valueMaxLen = defaultValueMaxLen
	}

	if o.formatter.itemSeparator == "" {
		o.formatter.itemSeparator = defaultListItemSeparator
	}

	return o.formatter
}

// FormatString - возвращает само значение value, т.к. оно уже строковое.
func (f *FieldFormatter) FormatString(value string) (string, error) {
	if len(value) > f.valueMaxLen {
		return "", errInternalValueLenMax.New(
			"curLength", len(value),
			"maxLength", f.valueMaxLen,
		)
	}

	return value, nil
}

// FormatStringList - возвращает список строк объединённую в одну строку через разделитель.
func (f *FieldFormatter) FormatStringList(values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}

	valuesLen := (len(values) - 1) * len(f.itemSeparator) // separator's len
	for i := range values {
		valuesLen += len(values[i])
	}

	if valuesLen > f.valueMaxLen {
		return "", errInternalValueLenMax.New(
			"curLength", valuesLen,
			"maxLength", f.valueMaxLen,
		)
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

// FormatInt64 - возвращает целое знаковое число в виде строки.
func (f *FieldFormatter) FormatInt64(value int64) (string, error) {
	return strconv.FormatInt(value, 10), nil
}

// FormatInt64List - возвращает список целых знаковых чисел объединённую в одну строку через разделитель.
func (f *FieldFormatter) FormatInt64List(values []int64) (string, error) {
	if len(values) == 0 {
		return "", nil
	}

	const maxInt64Digits = 19 // max number of digits in int64

	if len(values)*maxInt64Digits > f.valueMaxLen {
		return "", errors.NewInternalError(
			fmt.Sprintf(
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

// FormatBool - возвращает булево значение в виде строки.
func (f *FieldFormatter) FormatBool(value bool) (string, error) {
	return strconv.FormatBool(value), nil
}
