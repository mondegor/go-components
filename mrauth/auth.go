package mrauth

import (
	"context"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// OperationUseCase - подтверждение, повторная отправка кода и отзыв защищённой операции пользователя.
	OperationUseCase interface {
		ConfirmAction(ctx context.Context, operationToken, secret string) (secureoperation.SecureOperation, error)
		ResendCode(ctx context.Context, operationToken string) (secureoperation.SecureOperation, error)
		Revoke(ctx context.Context, operationToken string) error
	}

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
		Execute(ctx context.Context, userID uuid.UUID, payload []byte) error
	}

	// User2FAConfirmActionCreator - создаёт данные 2FA-подтверждения пользователя по его логину или идентификатору.
	User2FAConfirmActionCreator interface {
		CreateByUserLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (dto.User2FA, error)
		CreateByUserID(ctx context.Context, userID uuid.UUID) (dto.User2FA, error)
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

	// ConfirmByAddressCreator - создаёт действие подтверждения операции по контактному адресу (емаил/телефон).
	// Параметр confirmCode передаётся в открытом виде (для отправки) и в виде хеша (для хранения).
	ConfirmByAddressCreator interface {
		Create(address contactaddress.ContactAddress, confirmCode, hashedConfirmCode string) (secureoperation.ConfirmAction, error)
	}

	// SessionUseCase - управление открытыми сессиями текущего пользователя.
	SessionUseCase interface {
		GetList(ctx context.Context, userID uuid.UUID, currentAccessToken string) ([]dto.UserSession, error)
		Close(ctx context.Context, userID uuid.UUID, sessionIDs []uint32) error
	}
)

type (
	// AppResolver - определяет приложение и устройство по строке User-Agent.
	// Вход недоверенный (контролируется клиентом) - его нельзя писать в логи без
	// экранирования (CRLF/log-forging) и нельзя слепо подставлять во внешние запросы.
	AppResolver func(userAgent string) (appName, deviceName string)

	// LocationResolver - определяет местоположение по IP адресу.
	// Вход недоверенный (контролируется клиентом), см. предупреждение к AppResolver.
	LocationResolver func(ip string) string
)
