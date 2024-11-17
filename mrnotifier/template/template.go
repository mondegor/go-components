package template

import (
	"context"

	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

type (
	// UseCase - интерфейс получения шаблонов уведомлений.
	UseCase interface {
		GetItemByKey(ctx context.Context, name, lang string) (entity.Template, error)
	}

	// Storage - предоставляет доступ к хранилищу шаблонов уведомлений.
	Storage interface {
		FetchOneByKey(ctx context.Context, name, lang string) (entity.Template, error)
	}
)
