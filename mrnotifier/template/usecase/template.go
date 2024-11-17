package usecase

import (
	"context"

	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrlog"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/template"
	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

const (
	defaultDefaultLang = "en_EN"
)

type (
	// Template - объект для получения шаблонов уведомлений на основе которых
	// формируются персонализированные уведомления конкретным получателям.
	Template struct {
		storage      template.Storage
		errorWrapper mrcore.UseCaseErrorWrapper
		defaultLang  string
	}
)

// New - создаёт объект Template.
func New(
	storage template.Storage,
	errorWrapper mrcore.UseCaseErrorWrapper,
	defaultLang string,
) *Template {
	if defaultLang == "" {
		defaultLang = defaultDefaultLang
	}

	return &Template{
		storage:      storage,
		errorWrapper: errorWrapper,
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
		if lang != co.defaultLang && mrcore.ErrStorageNoRowFound.Is(err) {
			mrlog.Ctx(ctx).Warn().Msgf("No template was found for the notification %s with lang %s", key, lang)

			// если запись не найдена для указанного языка, то происходит попытка выбрать её с языком по умолчанию
			item, err = co.storage.FetchOneByKey(ctx, key, co.defaultLang)
		}

		if err != nil {
			if mrcore.ErrStorageNoRowFound.Is(err) {
				err = mrnotifier.ErrTemplateNotRegistered.Wrap(err, key, lang)
			}

			return entity.Template{}, co.errorWrapper.WrapErrorFailed(err, entity.ModelNameTemplate)
		}
	}

	return item, nil
}
