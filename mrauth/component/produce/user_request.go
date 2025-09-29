package produce

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// UserRequest - comment struct.
	UserRequest struct {
		producer       userLogProducer
		parserClientIP mrserver.RequestParserClientIP
		parserUser     mrserver.RequestParserUser
		logger         mrlog.Logger
	}

	userLogProducer interface {
		PushMessage(ctx context.Context, message []byte) error
	}
)

// NewUserRequest - создаёт объект UserRequest.
func NewUserRequest(
	producer userLogProducer,
	logger mrlog.Logger,
	parserClientIP mrserver.RequestParserClientIP,
	parserUser mrserver.RequestParserUser,
) *UserRequest {
	return &UserRequest{
		producer:       producer,
		parserClientIP: parserClientIP,
		parserUser:     parserUser,
		logger:         logger,
	}
}

// Enabled - comments method.
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

	activityLog := dto.UserActivityLog{
		UserID:        userID,
		UserIP:        rs.parserClientIP.DetailedIP(r),
		UserAgent:     r.UserAgent(),
		RequestPath:   r.URL.Path,
		RequestStatus: uint32(status), //nolint:gosec
		VisitedAt:     time.Now(),
	}

	bytes, err := json.Marshal(activityLog)
	if err != nil {
		rs.logger.Error(r.Context(), "UserRequest.Json.Marshal()", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = rs.producer.PushMessage(ctx, bytes)
	if err != nil {
		rs.logger.Error(r.Context(), "UserRequest.Producer.PushMessage()", "error", err)
	}
}
