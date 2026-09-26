package mrauth

import (
	"context"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
)

type (
	// AuthTokenFetcher - возвращает область действия пользователя по access токену.
	AuthTokenFetcher interface {
		FetchOneByAccessToken(ctx context.Context, accessToken string) (dto.UserScopes, error)
	}

	// UserStatisticUseCase - запись статистики активности пользователей.
	UserStatisticUseCase interface {
		Execute(ctx context.Context, list []dto.UserActivityLogMessage) error
	}

	// OperationHandler - обработчик прикладной логики, привязанной к защищённой операции.
	OperationHandler interface {
		Execute(ctx context.Context, actor dto.ActorMeta, payload []byte) error
	}

	// User2FAConfirmActionCreator - создаёт данные 2FA-подтверждения пользователя по его логину или идентификатору.
	User2FAConfirmActionCreator interface {
		CreateByUserLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (dto.User2FA, error)
		CreateByUserID(ctx context.Context, userID uuid.UUID) (dto.User2FA, error)
	}

	// RealmRegistry - реестр соответствия числового идентификатора realm его имени.
	// Имя используется на границах системы (HTTP, token scopes, отображение),
	// идентификатор - как компактный ключ хранения в БД.
	RealmRegistry interface {
		IDByName(name string) (id uint16, ok bool)
		NameByID(id uint16) (name string, ok bool)
	}

	// TokenGenerator - генератор случайных токенов заданной длины.
	TokenGenerator interface {
		GenToken() (string, error)
	}

	// CodeGenerator - генерация, хеширование и проверка кодов подтверждения.
	CodeGenerator interface {
		GenCodeWithHash() (code, hashedCode string, err error)
		HashedSecret(secret string) (string, error)
		CompareSecretAndHash(secret, hashedSecret string) (ok bool, err error)
	}

	// TokenIssuer - выпускает пару токенов access/refresh для области действия пользователя.
	TokenIssuer interface {
		CreateTokenPair(userScopes dto.UserScopes) (token dto.AuthTokenPair, err error)
	}

	// Notifier - ставит уведомление по ключу события в отправку; получатель передаётся
	// в props["to"] адресом (ID пользователя заменяется на email декоратором notify.UserEmailNotifier).
	Notifier interface {
		Send(ctx context.Context, key string, props map[string]any) error
	}

	// SessionUseCase - управление открытыми сессиями текущего пользователя.
	SessionUseCase interface {
		GetList(ctx context.Context, userID uuid.UUID, currentAccessToken, realm string) ([]dto.UserSession, error)
		Close(ctx context.Context, userID uuid.UUID, sessionIDs []uint32) error
	}
)
