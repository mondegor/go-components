package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// ApplySettings - изменение персональных настроек пользователя (язык и часовой пояс).
	ApplySettings struct {
		storage          userSettingsStorage
		langResolver     langResolver
		timeZoneResolver timeZoneResolver
		errorWrapper     errors.Wrapper
	}

	userSettingsStorage interface {
		UpdateSettings(ctx context.Context, row entity.UserSettings) error
	}

	// langResolver - подбирает язык, поддерживаемый приложением.
	langResolver interface {
		Resolve(lang string) (langCode string)
	}

	// timeZoneResolver - подбирает пояс, зарегистрированный в приложении.
	timeZoneResolver interface {
		Resolve(in dto.TimeZoneInfo) (name string)
	}
)

// NewApplySettings - создаёт объект ApplySettings.
func NewApplySettings(
	storage userSettingsStorage,
	langResolver langResolver,
	timeZoneResolver timeZoneResolver,
) *ApplySettings {
	return &ApplySettings{
		storage:          storage,
		langResolver:     langResolver,
		timeZoneResolver: timeZoneResolver,
		errorWrapper:     errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - обновляет язык и часовой пояс пользователя одним запросом и возвращает
// настройки, которые реально сохранены: часовой пояс может отличаться от запрошенного,
// если его имя приложению неизвестно и пояс был подобран по смещению, а язык - если он
// прислан в другой записи ("ru" вместо "ru-RU") или приложением не поддерживается.
// Вызывающий отдаёт результат клиенту, чтобы тот применил сохранённые значения у себя.
//
// Язык сохраняется только в том виде, в котором его отдаёт локализатор приложения:
// одна и та же локаль записывается по-разному, и без приведения в колонке накапливались бы
// разные записи одного языка.
func (uc *ApplySettings) Execute(ctx context.Context, userID uuid.UUID, settings dto.UserSettings) (dto.UserSettingsApplied, error) {
	item := entity.UserSettings{
		UserID:   userID,
		LangCode: uc.langResolver.Resolve(settings.LangCode),
		TimeZone: uc.timeZoneResolver.Resolve(settings.TimeZone),
	}

	if err := uc.storage.UpdateSettings(ctx, item); err != nil {
		return dto.UserSettingsApplied{}, uc.errorWrapper.Wrap(err)
	}

	return dto.UserSettingsApplied{
		LangCode: item.LangCode,
		TimeZone: item.TimeZone,
	}, nil
}
