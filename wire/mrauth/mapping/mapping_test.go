package mapping_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/mondegor/go-webcore/mrserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/wire/mrauth/mapping"
)

// TestOptionErrorCodeToHttpStatus - закрепляет контракт "код ошибки -> HTTP-статус" для сентинелов,
// которым в контракте объявлен статус, отличный от 400. Потерянная пара в OptionErrorCodeToHttpStatus
// молча вернёт ошибку в 400 и разойдётся со спекой, поэтому тест должен упасть раньше.
func TestOptionErrorCodeToHttpStatus(t *testing.T) {
	t.Parallel()

	statusMapper, err := mrserver.NewHttpErrorStatusMapper(0, mapping.OptionErrorCodeToHttpStatus()...)
	require.NoError(t, err)

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"токен неизвестен, истёк или уже использован", mrauth.ErrTokenNotFoundOrExpired, http.StatusUnauthorized},
		{"access токен не разобрался (битая подпись)", mrauth.ErrTokenInvalid, http.StatusForbidden},
		{"2FA уже включена", mrauth.ErrAuth2FAMustBeDisabledFirst, http.StatusConflict},
		{"2FA выключена", mrauth.ErrAuth2FAIsDisabled, http.StatusConflict},
		{"лимит сессий исчерпан", mrauth.ErrSessionLimitExceededTryLater, http.StatusTooManyRequests},
		{"регистрация уже идёт", mrauth.ErrSignupAlreadyInProgressTryLater, http.StatusTooManyRequests},
		{
			// срок повторной попытки прикладывается к сентинелу обёрткой, и наружу уходит именно
			// она: если обёртка потеряет код, троттл съедет с 429 на 400 и разойдётся со спекой
			"регистрация уже идёт, ошибка несёт срок повторной попытки",
			mrauth.ErrSignupAlreadyInProgressTryLater.Wrap(mrauth.NewRetryAfterError(10 * time.Minute)),
			http.StatusTooManyRequests,
		},
		{"пользовательская ошибка по умолчанию", mrauth.ErrLoginNotExists, http.StatusBadRequest},
		// ошибки защищённой операции в таблице отсутствуют намеренно: им нужен дефолтный 400
		// с кодом в теле. Появись здесь пара - и токен операции, переданный path-параметром
		// (`GET /v1/security/totp/{token}`), уехал бы клиенту статусом без кода вовсе
		{"токен операции недействителен", secureoperation.ErrOperationInvalid, http.StatusBadRequest},
		{"повторные отправки исчерпаны", secureoperation.ErrNoAttemptsToResendCode, http.StatusBadRequest},
		{"повторная отправка неприменима", secureoperation.ErrResendCodeIsNotSupported, http.StatusBadRequest},
		{"операция не подтверждена", secureoperation.ErrOperationIsNotConfirmed, http.StatusBadRequest},
		{"операция истекла", secureoperation.ErrOperationAlreadyExpired, http.StatusBadRequest},
		{"секрет не передан", secureoperation.ErrConfirmCodeIsRequired, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, statusMapper.ErrorStatus(tt.err))
		})
	}
}
