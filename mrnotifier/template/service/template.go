package service

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstatus/itemstatus"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/template/dto"
	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

const (
	defaultDefaultLang = "en-US"
)

type (
	// Template - объект для получения шаблонов уведомлений на основе которых
	// формируются персонализированные уведомления конкретным получателям.
	Template struct {
		storage      templateStorage
		storageVar   templateVarsStorage
		errorWrapper errors.Wrapper
		logger       mrlog.Logger
		defaultLang  string
	}

	templateStorage interface {
		FetchOneByKey(ctx context.Context, name, lang string) (row entity.Template, err error)
	}

	templateVarsStorage interface {
		Fetch(ctx context.Context, vars []string) ([]entity.Variable, error)
	}
)

// New - создаёт объект Template.
func New(
	storage templateStorage,
	storageVars templateVarsStorage,
	logger mrlog.Logger,
	defaultLang string,
) *Template {
	if defaultLang == "" {
		defaultLang = defaultDefaultLang
	}

	return &Template{
		storage:      storage,
		storageVar:   storageVars,
		errorWrapper: errors.NewServiceWrapper(),
		logger:       logger,
		defaultLang:  defaultLang,
	}
}

// GetItemByKey - возвращает шаблон уведомления по указанному имени (ключу) и языку.
// Если по указанному языку шаблон не был найден, то происходит попытка получения шаблона на языке по умолчанию.
func (uc *Template) GetItemByKey(ctx context.Context, key, lang string) (dto.Template, error) {
	if lang == "" {
		lang = uc.defaultLang
	}

	item, err := uc.getItemByKey(ctx, key, lang)
	if err != nil {
		if lang == uc.defaultLang {
			return dto.Template{}, uc.errorWrapper.Wrap(err)
		}

		// если запись не найдена для указанного языка, то происходит попытка выбрать её с языком по умолчанию
		item, err = uc.getItemByKey(ctx, key, uc.defaultLang)
		if err != nil {
			return dto.Template{}, uc.errorWrapper.Wrap(err)
		}
	}

	varRows, err := uc.getVars(ctx, item)
	if err != nil {
		return dto.Template{}, uc.errorWrapper.Wrap(err)
	}

	return dto.Template{
		Lang:  item.Lang,
		Props: item.Props,
		Vars:  varRows,
	}, nil
}

func (uc *Template) getItemByKey(ctx context.Context, key, lang string) (entity.Template, error) {
	item, err := uc.storage.FetchOneByKey(ctx, key, lang)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRowFound) {
			if lang == uc.defaultLang {
				return entity.Template{}, mrnotifier.ErrSystemTemplateNotRegistered.Wrap(err, "template", key, "lang", lang)
			}

			uc.logger.Warn(ctx, "No template was found for the notification", "template", key, "lang", lang)
		}

		return entity.Template{}, uc.errorWrapper.Wrap(err)
	}

	if item.Status != itemstatus.Enabled {
		if lang == uc.defaultLang {
			return entity.Template{}, mrnotifier.ErrSystemTemplateNotRegistered.Wrap(err, "template", key, "lang", lang, "status", item.Status)
		}

		uc.logger.Warn(ctx, "Template is not available for the notification", "template", key, "lang", lang, "template_status", item.Status)

		return entity.Template{}, errors.ErrEventStorageNoRowFound
	}

	return item, nil
}

func (uc *Template) getVars(ctx context.Context, row entity.Template) ([]entity.Variable, error) {
	if len(row.Vars) == 0 {
		return nil, nil
	}

	varRows, err := uc.storageVar.Fetch(ctx, row.Vars)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	if len(row.Vars) != len(varRows) {
		uc.logger.Warn(
			ctx,
			"incorrect template vars count",
			"vars_count", len(varRows),
			"vars_expected", len(row.Vars),
			"template", row.Name,
			"lang", row.Lang,
		)
	}

	return varRows, nil
}
