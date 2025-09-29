package fieldparser

type (
	// Option - настройка объекта DBFieldParser.
	Option func(p *DBFieldParser)
)

// WithItemSeparator - устанавливает опцию itemSeparator для DBFieldParser.
func WithItemSeparator(value string) Option {
	return func(p *DBFieldParser) {
		if value != "" {
			p.itemSeparator = value
		}
	}
}
