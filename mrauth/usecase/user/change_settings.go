package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// ChangeSettings - изменение персональных настроек пользователя (язык и часовой пояс).
	ChangeSettings struct {
		txManager        mrstorage.DBTxManager
		storage          userSettingsStorage
		storageAuthToken authTokenSettingsStorage
		errorWrapper     errors.Wrapper
	}

	userSettingsStorage interface {
		UpdateSettings(ctx context.Context, row entity.UserSettings) error
	}

	authTokenSettingsStorage interface {
		UpdateScopesSettings(ctx context.Context, userID uuid.UUID, langCode, timeZone string) error
	}
)

// NewChangeSettings - создаёт объект ChangeSettings.
func NewChangeSettings(
	txManager mrstorage.DBTxManager,
	storage userSettingsStorage,
	storageAuthToken authTokenSettingsStorage,
) *ChangeSettings {
	return &ChangeSettings{
		txManager:        txManager,
		storage:          storage,
		storageAuthToken: storageAuthToken,
		errorWrapper:     errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - сохраняет язык и часовой пояс в профиль пользователя и переносит их в область
// действия всех его действующих refresh токенов, чтобы ближайшее продление сессии выпустило
// пару токенов уже с новыми настройками.
//
// Значения сохраняются как есть: приводить их не к чему, на границе ввода принимаются только
// язык и пояс, зарегистрированные приложением (см. model.ChangeSettingsRequest).
//
// Оба обновления идут одной транзакцией: профиль не должен расходиться с областью действия токенов.
func (uc *ChangeSettings) Execute(ctx context.Context, userID uuid.UUID, langCode, timeZone string) error {
	item := entity.UserSettings{
		UserID:   userID,
		LangCode: langCode,
		TimeZone: timeZone,
	}

	err := uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.UpdateSettings(ctx, item); err != nil {
			return err
		}

		return uc.storageAuthToken.UpdateScopesSettings(ctx, item.UserID, item.LangCode, item.TimeZone)
	})
	if err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
