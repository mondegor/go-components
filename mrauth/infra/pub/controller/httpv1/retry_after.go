package httpv1

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/mondegor/go-components/mrauth"
)

const (
	// headerKeyRetryAfter - заголовок с подсказкой, через сколько повторять временно отклонённый запрос.
	headerKeyRetryAfter = "Retry-After"
)

// setRetryAfterHeader - выставляет заголовок Retry-After в форме delay-seconds (RFC 9110).
// Заголовок должен ставится до возврата ошибки из обработчика.
func setRetryAfterHeader(w http.ResponseWriter, delay time.Duration) {
	if delay <= 0 {
		return
	}

	w.Header().Set(headerKeyRetryAfter, strconv.FormatInt(int64(math.Ceil(delay.Seconds())), 10))
}

// setRetryAfterHeaderFromError - выставляет заголовок Retry-After, если ошибка принесла срок повторной попытки.
func setRetryAfterHeaderFromError(w http.ResponseWriter, err error) {
	var retryErr *mrauth.RetryAfterError

	if errors.As(err, &retryErr) {
		setRetryAfterHeader(w, retryErr.RetryAfter)
	}
}
