package usecase

import (
	"context"
	"fmt"

	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrlog"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/template"
	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

const (
	defaultDefaultLang = "en-US"
)

type (
	// Template - объект для получения шаблонов уведомлений на основе которых
	// формируются персонализированные уведомления конкретным получателям.
	Template struct {
		storage      template.Storage
		errorWrapper core.UseCaseErrorWrapper
		logger       mrlog.Logger
		defaultLang  string
	}
)

// New - создаёт объект Template.
func New(
	storage template.Storage,
	logger mrlog.Logger,
	defaultLang string,
) *Template {
	if defaultLang == "" {
		defaultLang = defaultDefaultLang
	}

	return &Template{
		storage:      storage,
		errorWrapper: core.NewUseCaseErrorWrapper(entity.ModelNameTemplate),
		logger:       logger,
		defaultLang:  defaultLang,
	}
}

// GetItemByKey - возвращает шаблон уведомления по указанному имени (ключу) и языку.
// Если по указанному языку шаблон не был найден, то происходит попытка получения шаблона на языке по умолчанию.
func (co *Template) GetItemByKey(ctx context.Context, key, lang string) (entity.Template, error) {
	if lang == "" {
		lang = co.defaultLang
	}

	item, err := co.storage.FetchOneByKey(ctx, key, lang)
	if err != nil {
		if lang != co.defaultLang && mr.ErrStorageNoRowFound.Is(err) {
			co.logger.Warn(ctx, fmt.Sprintf("No template was found for the notification %s with lang %s", key, lang))

			// если запись не найдена для указанного языка, то происходит попытка выбрать её с языком по умолчанию
			item, err = co.storage.FetchOneByKey(ctx, key, co.defaultLang)
		}

		if err != nil {
			if mr.ErrStorageNoRowFound.Is(err) {
				err = mrnotifier.ErrTemplateNotRegistered.Wrap(err, key, lang)
			}

			return entity.Template{}, co.errorWrapper.WrapErrorFailed(err)
		}
	}

	return item, nil
}
