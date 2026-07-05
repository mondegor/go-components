package collector

import (
	"github.com/mondegor/go-core/mrprocess/collect"

	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// Option - настройка объекта ComponentService.
	Option func(o *options)

	options struct {
		collectorOpts []collect.Option[dto.UserActivityLogMessage]
	}
)

// WithMessageCollectorOpts - устанавливает опцию collectorOpts для ComponentService.
func WithMessageCollectorOpts(value ...collect.Option[dto.UserActivityLogMessage]) Option {
	return func(o *options) {
		o.collectorOpts = append(o.collectorOpts, value...)
	}
}
