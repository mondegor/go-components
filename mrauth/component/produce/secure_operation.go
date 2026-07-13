package produce

import (
	"context"
	"time"

	"github.com/mondegor/go-core/mrlog"

	"github.com/mondegor/go-components/mrauth/entity"
)

// Справочник «Событие → Status → Reason» (Status - enum logstatus, Reason - enum logreason).
// Первая колонка - семантика события (без привязки к file:line, чтобы карта не устаревала при перемещении кода):
//
//	| Событие                                                        | Status          | Reason             |
//	|----------------------------------------------------------------|-----------------|--------------------|
//	| Инициация входа (создание сессии)                              | OPENED          | -                  |
//	| Вход: логин не существует                                      | BLOCKED         | LOGIN_NOT_EXISTS   |
//	| Инициация регистрации                                          | OPENED          | -                  |
//	| Регистрация: анти-спам лок                                     | BLOCKED         | THROTTLED          |
//	| Повторная отправка кода подтверждения                          | RESENT_CODE     | -                  |
//	| Повторная отправка: троттлинг                                  | BLOCKED         | THROTTLED          |
//	| Подтверждение: неверный код                                    | CONFIRM_FAILED  | WRONG_CODE         |
//	| Подтверждение: исчерпаны попытки                               | BLOCKED         | ATTEMPTS_EXHAUSTED |
//	| Подтверждение: гонка 2FA / повтор TOTP-шага                    | CONFIRM_FAILED  | TOTP_REPLAY        |
//	| Подтверждение промежуточного действия успешно                  | CONFIRM_SUCCESS | -                  |
//	| Операция полностью подтверждена                                | CONFIRMED       | -                  |
//	| Операция отозвана                                              | REVOKED         | -                  |
//	| Операция применена                                             | APPLIED         | -                  |
//	| Применение: обращение к чужой операции (владелец не совпал)    | BLOCKED         | ACCESS_FORBIDDEN   |
//	| Применение: операция ещё не подтверждена                       | BLOCKED         | NOT_CONFIRMED      |
//	| Применение TOTP: неверный код                                  | CONFIRM_FAILED  | WRONG_CODE         |
//	| Создание защ. операции (смена email/phone/totp/password,       | OPENED          | -                  |
//	|   отключение 2FA, перегенерация recovery)                      |                 |                    |
//	| Вход выполнен (сессия открыта, токены выданы)                  | SESSION_OPENED  | -                  |
//	| Вход: жёсткий лимит сессий                                     | BLOCKED         | SESSION_LIMIT      |
//	| Повторное использование refresh-токена (атака)                 | BLOCKED         | TOKEN_REUSE        |
//
// reason=UNSPECIFIED (0) используется при успешных исходах. У pre-op событий BLOCKED (до создания операции)
// confirm_method=UNSPECIFIED, а operation_name берётся у фабрики операции (метод Name), чтобы такие события
// не разъезжались с именем самой операции. Исключение - продление сессии (session.continue): в этом потоке
// операции нет вообще, поэтому имя задаётся константой usecase/session.
//
// Запись журнала собирается конструктором entity.NewSecureOperationLog (он же фиксирует время события).

const (
	// pushTimeout - ограничение ожидания места в очереди коллектора (общее для продюсеров пакета).
	// PushMessage блокируется, пока очередь заполнена, поэтому таймаут держится небольшим:
	// журнал best-effort и не должен добавлять заметную задержку к обработке запроса,
	// при заторе запись теряется с логированием ошибки.
	pushTimeout = 300 * time.Millisecond
)

type (
	// SecureOperationLogger - best-effort продюсер записей журнала защищённых операций:
	// пушит запись в коллектор, ошибку логирует, но не возвращает (журнал не должен ронять операцию).
	SecureOperationLogger struct {
		producer secureOperationLogProducer
		logger   mrlog.Logger
	}

	secureOperationLogProducer interface {
		PushMessage(ctx context.Context, entry entity.SecureOperationLog) error
	}

	// noopLogProducer - продюсер-заглушка для хостов, не поднявших коллектор журнала.
	noopLogProducer struct{}
)

// NewSecureOperationLogger - создаёт объект SecureOperationLogger.
// Если продюсер не указан, то журнал молча отключается (запись никуда не пишется).
func NewSecureOperationLogger(
	producer secureOperationLogProducer,
	logger mrlog.Logger,
) *SecureOperationLogger {
	if producer == nil {
		producer = noopLogProducer{}
	}

	return &SecureOperationLogger{
		producer: producer,
		logger:   logger,
	}
}

// PushMessage - принимает и отбрасывает запись журнала.
func (noopLogProducer) PushMessage(_ context.Context, _ entity.SecureOperationLog) error {
	return nil
}

// Log - фиксирует запись журнала защищённых операций.
// Использует отдельный контекст (не контекст запроса), чтобы отменённый запрос не срывал запись,
// но ограничивает ожидание pushTimeout'ом, т.к. вызывается из горутины запроса.
// Ошибку логирует, но не возвращает - сбой журнала не должен прерывать основную операцию.
func (rs *SecureOperationLogger) Log(ctx context.Context, entry entity.SecureOperationLog) {
	pushCtx, cancel := context.WithTimeout(context.Background(), pushTimeout)
	defer cancel()

	if err := rs.producer.PushMessage(pushCtx, entry); err != nil {
		rs.logger.Error(ctx, "SecureOperationLogger.Log()", "error", err)
	}
}
