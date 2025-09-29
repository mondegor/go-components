package collector

import (
	"github.com/mondegor/go-webcore/mrworker/process/collect"
)

type (
	// ServiceOption - настройка объекта ComponentService.
	ServiceOption func(o *serviceOptions)
)

// WithMessageCollectorOpts - устанавливает опцию requestCollector для ComponentService.
func WithMessageCollectorOpts(value ...collect.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.requestCollector = append(o.requestCollector, value...)
		}
	}
}
