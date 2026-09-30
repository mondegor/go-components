package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
)

type (
	// ActorMeta - метаданные клиента для журнала защищённых операций и уведомлений о них.
	ActorMeta struct {
		// UserID - для залогиненных потоков равен userID, для анонимных - uuid.Nil (форензику несёт ClientIP).
		UserID uuid.UUID

		// ClientIP - недоверенный ввод, контролируемый клиентом.
		ClientIP mrtype.DetailedIP

		// UserAgent - недоверенный ввод, контролируемый клиентом.
		UserAgent string

		location *time.Location
	}
)

// NewActorMeta - создаёт ActorMeta с часовым поясом клиента location (см. Location).
func NewActorMeta(userID uuid.UUID, clientIP mrtype.DetailedIP, userAgent string, location *time.Location) ActorMeta {
	return ActorMeta{
		UserID:    userID,
		ClientIP:  clientIP,
		UserAgent: userAgent,
		location:  location,
	}
}

// NewAnonymousActorMeta - создаёт ActorMeta анонимного клиента (UserID = uuid.Nil): владелец
// становится известен позже (см. WithUser), форензику до этого несёт ClientIP.
func NewAnonymousActorMeta(clientIP mrtype.DetailedIP, userAgent string, location *time.Location) ActorMeta {
	return NewActorMeta(uuid.Nil, clientIP, userAgent, location)
}

// WithUser - возвращает копию метаданных с указанным пользователем.
// Применяется в анонимных потоках, когда владелец операции становится известен после её чтения:
// нулевой userID игнорируется (у операции регистрации владельца ещё нет - запись остаётся анонимной).
func (m ActorMeta) WithUser(userID uuid.UUID) ActorMeta {
	if userID != uuid.Nil {
		m.UserID = userID
	}

	return m
}

// Location - возвращает часовой пояс клиента, в котором ему выводится время.
// Если пояс не задан (в том числе у ActorMeta, созданного без конструктора), возвращается UTC.
func (m ActorMeta) Location() *time.Location {
	if m.location == nil {
		return time.UTC
	}

	return m.location
}

// NewOperationLog - собирает запись журнала защищённых операций от имени этого актора
// (подставляет UserID и ClientIP), фиксируя остальные поля события.
// TODO: скорее всего нужно сделать хелпер функцию, а не метод.
func (m ActorMeta) NewOperationLog(
	sourceName string,
	method confirmmethod.Enum,
	status logstatus.Enum,
	reason logreason.Enum,
) entity.SecureOperationLog {
	return entity.NewSecureOperationLog(m.UserID, m.ClientIP, sourceName, method, status, reason)
}
