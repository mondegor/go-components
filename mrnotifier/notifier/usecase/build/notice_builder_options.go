package build

type (
	// Option - настройка объекта NoticeSender.
	Option func(co *NoticeBuilder)
)

// WithChannelPrefix - устанавливает опцию channelPrefix для NoticeBuilder.
func WithChannelPrefix(value string) Option {
	return func(co *NoticeBuilder) {
		co.channelPrefix = value
	}
}
