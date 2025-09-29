package processor

import (
	"github.com/mondegor/go-webcore/mrworker/process/consume"
)

type (
	// ServiceOption - настройка объекта ComponentService.
	ServiceOption func(o *serviceOptions)
)

// WithDefaultLang - устанавливает опцию defaultLang для ComponentService.
func WithDefaultLang(value string) ServiceOption {
	return func(o *serviceOptions) {
		if o.defaultLang != "" {
			o.defaultLang = value
		}
	}
}

// WithNoticeProcessorOpts - устанавливает опцию noticeProcessor для ComponentService.
func WithNoticeProcessorOpts(value ...consume.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.noticeProcessor = append(o.noticeProcessor, value...)
		}
	}
}
