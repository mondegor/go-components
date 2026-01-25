package usecase

type (
	// Option - настройка объекта BuildNotice.
	Option func(o *options)

	options struct {
		builder *BuildNotice
	}
)

// WithChannelPrefix - устанавливает опцию channelPrefix для BuildNotice.
func WithChannelPrefix(value string) Option {
	return func(o *options) {
		o.builder.channelPrefix = value
	}
}
