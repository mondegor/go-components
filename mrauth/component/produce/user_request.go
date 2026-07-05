package produce

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-webcore/mrserver/request"

	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// UserRequest - трассировщик http-запросов: формирует сообщение об активности пользователя
	// и отправляет его в очередь статистики.
	UserRequest struct {
		producer       userLogProducer
		parserClientIP request.ParserClientIP
		parserUser     request.ParserUser
		logger         mrlog.Logger
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
) *UserRequest {
	return &UserRequest{
		producer:       producer,
		parserClientIP: parserClientIP,
		parserUser:     parserUser,
		logger:         logger,
	}
}

// Enabled - сообщает, что трассировщик включён (всегда true).
func (rs *UserRequest) Enabled() bool {
	return true
}

// Emit - функция трассировки http запроса.
func (rs *UserRequest) Emit(r *http.Request, _ []byte, _ int, _ []byte, _ int, _ time.Duration, status int) {
	userID := rs.parserUser.UserID(r)
	if userID == uuid.Nil {
		return
	}

	if status < 0 {
		rs.logger.Error(r.Context(), "UserRequest.status is negative", "status", status)
		status = 0
	}

	activityLog := dto.UserActivityLogMessage{
		UserID:        userID,
		SessionID:     rs.parseSessionID(r.Context(), rs.parserUser.SessionID(r)),
		UserIP:        rs.parserClientIP.DetailedIP(r),
		UserAgent:     r.UserAgent(),
		RequestPath:   r.URL.Path,
		RequestStatus: uint32(status),
		VisitedAt:     time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rs.producer.PushMessage(ctx, activityLog); err != nil {
		rs.logger.Error(r.Context(), "UserRequest.Producer.PushMessage()", "error", err)
	}
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
