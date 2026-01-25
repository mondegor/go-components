package handle

type (
	// Option - настройка объекта RequestHandler.
	Option func(o *options)

	options struct {
		handler *RequestHandler
	}
)
