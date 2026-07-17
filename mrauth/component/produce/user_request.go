package produce

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-webcore/mrserver/request"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/usergroup"
)

const (
	// unknownRealmLogPeriod - минимальный интервал между сообщениями о неизвестном realm'е.
	// Emit вызывается на каждый http-ответ, поэтому лог без ограничения залил бы вывод; но и
	// разовое сообщение не годится - оно навсегда скрыло бы разъезд реестра, случившийся позже.
	unknownRealmLogPeriod = 1 * time.Hour
)

type (
	// UserRequest - трассировщик http-запросов: формирует сообщение об активности пользователя
	// и отправляет его в очередь статистики. Запросы анонимных пользователей не учитываются;
	// запросы с неизвестным realm'ом уходят с RealmID = 0 (сессия и журнал сохраняются,
	// статистика по realm'у не ведётся, см. dto.UserActivityLogMessage.RealmID).
	UserRequest struct {
		producer       userLogProducer
		parserClientIP request.ParserClientIP
		parserUser     request.ParserUser
		realmRegistry  mrauth.RealmRegistry
		logger         mrlog.Logger

		// unknownRealmLogAt - время (unix nano), начиная с которого о неизвестном realm'е
		// можно сообщить снова; нулевое значение разрешает сообщить немедленно
		unknownRealmLogAt atomic.Int64
	}

	userLogProducer interface {
		PushMessage(ctx context.Context, message dto.UserActivityLogMessage) error
	}
)

// NewUserRequest - создаёт объект UserRequest.
func NewUserRequest(
	producer userLogProducer,
	logger mrlog.Logger,
	parserClientIP request.ParserClientIP,
	parserUser request.ParserUser,
	realmRegistry mrauth.RealmRegistry,
) *UserRequest {
	return &UserRequest{
		producer:       producer,
		parserClientIP: parserClientIP,
		parserUser:     parserUser,
		realmRegistry:  realmRegistry,
		logger:         logger,
	}
}

// Enabled - сообщает, что трассировщик включён (всегда true).
func (rs *UserRequest) Enabled() bool {
	return true
}

// Emit - функция трассировки http запроса.
func (rs *UserRequest) Emit(r *http.Request, _ []byte, _ int, _ []byte, _ int, _ time.Duration, status int) {
	userID, group := rs.parserUser.UserAndGroup(r)
	if userID == uuid.Nil {
		return
	}

	realmID, ok := rs.realmRegistry.IDByName(usergroup.Realm(group))
	if !ok {
		// ошибка конфигурации: реестр realm'ов разошёлся с провайдерами. Сообщение не дропается,
		// а уходит с сентинелом RealmID = 0: иначе замёрз бы keep-alive сессий (sessions.updated_at),
		// и session-limit eviction закрывал бы реально активные сессии; теряется только
		// per-realm статистика (свёртка last-visited пропускает realm 0)
		realmID = 0 // явно: контракт IDByName не обязывает возвращать 0 при промахе

		if rs.allowUnknownRealmLog(time.Now()) {
			rs.logger.Error(r.Context(), "UserRequest.realm is unknown", "userId", userID, "group", group)
		}
	}

	if status < 0 {
		rs.logger.Error(r.Context(), "UserRequest.status is negative", "status", status)
		status = 0
	}

	activityLog := dto.UserActivityLogMessage{
		UserID:    userID,
		RealmID:   realmID,
		SessionID: rs.parseSessionID(r.Context(), rs.parserUser.SessionID(r)),
		// инвариант: real IP всегда задан - источник RemoteAddr, который в поддерживаемых
		// конфигурациях (tcp-listener) всегда парсится; на этом держатся NOT NULL колонки
		// users_activity_log.user_ip и sessions.last_ip, куда сообщение попадает из очереди
		UserIP:        rs.parserClientIP.DetailedIP(r),
		UserAgent:     r.UserAgent(),
		RequestPath:   r.URL.Path,
		RequestStatus: uint32(status),
		VisitedAt:     time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
	defer cancel()

	if err := rs.producer.PushMessage(ctx, activityLog); err != nil {
		rs.logger.Error(r.Context(), "UserRequest.Producer.PushMessage()", "error", err)
	}
}

// allowUnknownRealmLog - сообщает, пора ли снова писать в лог о неизвестном realm'е,
// и сдвигает срок следующего сообщения на unknownRealmLogPeriod вперёд.
// Срок общий для всех realm'ов: сообщение указывает на ошибку конфигурации, разбирать
// которую всё равно придётся целиком, а конкретная группа попадает в текст сообщения.
func (rs *UserRequest) allowUnknownRealmLog(now time.Time) bool {
	nextAt := rs.unknownRealmLogAt.Load()
	if now.UnixNano() < nextAt {
		return false
	}

	// CAS, а не Store: при гонке право записать лог получает ровно одна горутина,
	// проигравшие пропускают сообщение до следующего срока
	return rs.unknownRealmLogAt.CompareAndSwap(nextAt, now.Add(unknownRealmLogPeriod).UnixNano())
}

// parseSessionID - преобразует строковый идентификатор сессии из заголовка в uint32;
// для пустого/некорректного значения возвращает 0 (запрос без привязки к сессии).
func (rs *UserRequest) parseSessionID(ctx context.Context, raw string) uint32 {
	// запрос без привязки к сессии (например JWT без 'sid')
	if raw == "" {
		return 0
	}

	sessionID, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		rs.logger.Error(ctx, "error parsing sessionID", "error", err)

		return 0
	}

	return uint32(sessionID)
}
