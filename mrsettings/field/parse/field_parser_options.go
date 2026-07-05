package parse

type (
	// Option - настройка объекта FieldParser.
	Option func(o *options)

	options struct {
		parser *FieldParser
	}
)

// WithItemSeparator - устанавливает опцию itemSeparator для FieldParser.
func WithItemSeparator(value string) Option {
	return func(o *options) {
		o.parser.itemSeparator = value
	}
}
