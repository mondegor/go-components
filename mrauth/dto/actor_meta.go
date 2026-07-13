package dto

import (
	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
)

type (
	// ActorMeta - метаданные клиента для журнала защищённых операций.
	ActorMeta struct {
		// VisitorID - для залогиненных потоков равен userID, для анонимных - uuid.Nil (форензику несёт ClientIP).
		VisitorID uuid.UUID

		// ClientIP - недоверенный ввод, контролируемый клиентом.
		ClientIP mrtype.DetailedIP
	}
)

// WithVisitor - возвращает копию метаданных с указанным посетителем.
// Применяется в анонимных потоках, когда владелец операции становится известен после её чтения:
// нулевой userID игнорируется (у операции регистрации владельца ещё нет - запись остаётся анонимной).
func (m ActorMeta) WithVisitor(userID uuid.UUID) ActorMeta {
	if userID != uuid.Nil {
		m.VisitorID = userID
	}

	return m
}

// NewOperationLog - собирает запись журнала защищённых операций от имени этого актора
// (подставляет VisitorID и ClientIP), фиксируя остальные поля события.
// TODO: скорее всего нужно сделать хелпер функцию, а не метод.
func (m ActorMeta) NewOperationLog(
	operationName string,
	method confirmmethod.Enum,
	status logstatus.Enum,
	reason logreason.Enum,
) entity.SecureOperationLog {
	return entity.NewSecureOperationLog(m.VisitorID, m.ClientIP, operationName, method, status, reason)
}
