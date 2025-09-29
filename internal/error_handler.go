package core

import (
	"context"

	"github.com/mondegor/go-sysmess/mrerr"
)

//go:generate mockgen -source=error_handler.go -destination=./mock/error_handler.go

type (
	// ErrorHandler - обработчик ошибок.
	ErrorHandler interface {
		Handle(ctx context.Context, err error)
		HandleWith(ctx context.Context, err error, extraHandler func(analyzedKind mrerr.ErrorKind, err error))
	}
)
