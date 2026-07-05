package format

type (
	// Option - настройка объекта FieldFormatter.
	Option func(o *options)

	options struct {
		formatter *FieldFormatter
	}
)

// WithValueMaxLen - устанавливает опцию valueMaxLen для FieldFormatter.
func WithValueMaxLen(value int) Option {
	return func(o *options) {
		o.formatter.valueMaxLen = value
	}
}

// WithListItemSeparator - устанавливает опцию itemSeparator для FieldFormatter.
func WithListItemSeparator(value string) Option {
	return func(o *options) {
		o.formatter.itemSeparator = value
	}
}
