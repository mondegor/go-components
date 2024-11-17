package fieldformatter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mondegor/go-webcore/mrcore"
)

const (
	defaultValueMaxLen       = 65536
	defaultListItemSeparator = ","
)

type (
	// DBFieldFormatter - объект для преобразования данных поступающих
	// из внешнего источника в формат для сохранения в хранилище данных.
	DBFieldFormatter struct {
		valueMaxLen   uint32
		itemSeparator string
	}
)

// New - создаёт объект DBFieldFormatter.
func New(opts ...Option) *DBFieldFormatter {
	f := &DBFieldFormatter{
		valueMaxLen:   defaultValueMaxLen,
		itemSeparator: defaultListItemSeparator,
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// FormatString - возвращает само значение value, т.к. оно уже строковое.
func (f *DBFieldFormatter) FormatString(value string) (string, error) {
	if uint32(len(value)) > f.valueMaxLen {
		return "", mrcore.ErrInternalValueLenMax.New(len(value), f.valueMaxLen)
	}

	return value, nil
}

// FormatStringList - возвращает список строк объединённую в одну строку через разделитель.
func (f *DBFieldFormatter) FormatStringList(values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}

	valuesLen := (len(values) - 1) * len(f.itemSeparator) // separator's len
	for i := range values {
		valuesLen += len(values[i])
	}

	if uint32(valuesLen) > f.valueMaxLen {
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

// FormatInt64 - возвращает целое знаковое число в виде строки.
func (f *DBFieldFormatter) FormatInt64(value int64) (string, error) {
	return strconv.FormatInt(value, 10), nil
}

// FormatInt64List - возвращает список целых знаковых чисел объединённую в одну строку через разделитель.
func (f *DBFieldFormatter) FormatInt64List(values []int64) (string, error) {
	if len(values) == 0 {
		return "", nil
	}

	const maxInt64Digits = 19 // max number of digits in int64

	if uint32(len(values)*maxInt64Digits) > f.valueMaxLen {
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

// FormatBool - возвращает булево значение в виде строки.
func (f *DBFieldFormatter) FormatBool(value bool) (string, error) {
	return strconv.FormatBool(value), nil
}
