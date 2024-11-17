package fieldformatter

type (
	// Option - настройка объекта DBFieldFormatter.
	Option func(p *DBFieldFormatter)
)

// WithValueMaxLen - устанавливает опцию valueMaxLen для DBFieldFormatter.
func WithValueMaxLen(value uint32) Option {
	return func(p *DBFieldFormatter) {
		if p.valueMaxLen > 0 {
			p.valueMaxLen = value
		}
	}
}

// WithListItemSeparator - устанавливает опцию itemSeparator для DBFieldFormatter.
func WithListItemSeparator(value string) Option {
	return func(p *DBFieldFormatter) {
		if value != "" {
			p.itemSeparator = value
		}
	}
}
