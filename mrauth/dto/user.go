package dto

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
)

type (
	// User - информация о группе, языке и часовом поясе пользователя.
	User struct {
		ID       uuid.UUID
		Group    string
		LangCode string
		TimeZone string
	}

	// UserScopes - область действия пользователя.
	// Теги json синхронизированы с entity.AuthTokenScopes: хранилище токенов сохраняет
	// и читает области действия в этом формате; UserID/SessionID хранятся отдельно
	// и в сериализацию не входят.
	UserScopes struct {
		UserID    uuid.UUID `json:"-"`
		SessionID uint32    `json:"-"`
		Realm     string    `json:"realm"` // domain + '/' + user_group
		Kind      string    `json:"kind"`
		LangCode  string    `json:"lang"`
		TimeZone  string    `json:"tz"`
		// Email    string
		// Phone    uint64
	}

	// UserActivityLastVisited - информация о последнем посещении пользователя в рамках realm'а.
	UserActivityLastVisited struct {
		UserID        uuid.UUID
		RealmID       uint16
		LastVisitedAt time.Time
	}

	// SessionLastActivity - информация о последней активности сессии (для async обновления).
	SessionLastActivity struct {
		UserID        uuid.UUID
		SessionID     uint32
		LastIP        netip.Addr
		LastVisitedAt time.Time
	}

	// UserInfo - сгруппированная информация о пользователе.
	UserInfo struct {
		User              entity.User
		Auth2FA           entity.Auth2FA
		Realms            []UserRealmInfo
		PendingOperations []PendingOperation
	}

	// PendingOperation - действующая операция личного кабинета пользователя, ожидающая
	// подтверждения либо применения. NewEmail - у операций смены емаила (оба шага);
	// CurrentAction - только у неподтверждённой операции.
	PendingOperation struct {
		Token         string
		Type          operationtype.Enum
		Status        operationstatus.Enum
		ExpiresAt     time.Time
		NewEmail      string
		CurrentAction *PendingAction
	}

	// PendingAction - текущее звено неподтверждённой операции. RemainingResends и ResendsAt
	// заданы, только если звено допускает повторную отправку кода.
	PendingAction struct {
		Method            confirmmethod.Enum
		RemainingAttempts int16
		RemainingResends  *int16
		ResendsAt         *time.Time
	}

	// UserRealmInfo - привязка пользователя к realm'у вместе со статистикой последнего входа.
	// LastLocation - местоположение последнего входа (человекочитаемый IP, если не резолвится);
	// LastLoggedAt - время последнего входа в этот realm.
	UserRealmInfo struct {
		RealmID      uint16
		Kind         string
		LastLocation string
		LastLoggedAt time.Time
		CreatedAt    time.Time
		UpdatedAt    time.Time
	}

	// UserActivityLogMessage - информация об активности пользователя.
	// RealmID = 0 - сентинел "realm не определён" (реестр realm'ов разошёлся с провайдерами
	// пользователей, см. collect.UserRequest.Emit): сессия и журнал обрабатываются как обычно,
	// per-realm статистика для такого сообщения не ведётся. В конфиге realm id = 0 запрещён
	// (config.ValidateRealms), поэтому с настоящим realm'ом сентинел не пересекается.
	UserActivityLogMessage struct {
		UserID        uuid.UUID         `json:"user_id"`
		RealmID       uint16            `json:"realm_id"`
		SessionID     uint32            `json:"session_id"`
		UserIP        mrtype.DetailedIP `json:"user_ip"`
		UserAgent     string            `json:"user_agent"`
		RequestPath   string            `json:"request_path"`
		RequestStatus uint32            `json:"request_status"`
		VisitedAt     time.Time         `json:"visited_at"`
	}
)

// Validate - проверяет, что область действия пригодна для выпуска токена: язык и часовой
// пояс должны быть заданы. Оба попадают в токен и оттуда в профиль запроса, а токен с пустой
// секцией при разборе считается невалидным, поэтому иначе был бы выпущен токен, не проходящий
// собственную же проверку.
func (s UserScopes) Validate() error {
	if s.LangCode == "" {
		return fmt.Errorf("userScopes: langCode is empty (userId='%s')", s.UserID)
	}

	if s.TimeZone == "" {
		return fmt.Errorf("userScopes: timeZone is empty (userId='%s')", s.UserID)
	}

	return nil
}
