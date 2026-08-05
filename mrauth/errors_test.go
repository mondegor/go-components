package mrauth_test

import (
	"testing"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/errors/kind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth"
)

// TestRetryAfterErrorKeepsUserError - срок повторной попытки прикладывается к пользовательской
// ошибке её же методом Wrap, поэтому для приложения ошибка обязана остаться прежней: тот же код
// и тот же kind.User. Иначе временный отказ съехал бы с 429 на 500, а тело ответа - на другой код.
func TestRetryAfterErrorKeepsUserError(t *testing.T) {
	t.Parallel()

	err := mrauth.ErrSignupAlreadyInProgressTryLater.Wrap(mrauth.NewRetryAfterError(10 * time.Minute))

	require.ErrorIs(t, err, mrauth.ErrSignupAlreadyInProgressTryLater)
	require.NotErrorIs(t, err, mrauth.ErrEmailAlreadyExists)

	// код и тип берутся прямым приведением типа (так их читают errors.WithCustomCode
	// и обработчик ошибок приложения), а не через цепочку Unwrap
	userErr, ok := err.(interface {
		Kind() kind.Enum
		Code() string
	})
	require.True(t, ok, "обёрнутая ошибка должна остаться пользовательской ошибкой с кодом")
	assert.Equal(t, kind.User, userErr.Kind())
	assert.Equal(t, "SignupAlreadyInProgressTryLater", userErr.Code())

	assert.True(t, errors.WithCustomCode(err, "user_email").IsKindUser())
}

// TestRetryAfterErrorCarriesDelay - контроллер достаёт срок из ошибки через errors.As,
// поэтому он обязан находиться в цепочке в неизменном виде.
func TestRetryAfterErrorCarriesDelay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want time.Duration
	}{
		{
			name: "срок приложен к сентинелу",
			err:  mrauth.ErrSignupAlreadyInProgressTryLater.Wrap(mrauth.NewRetryAfterError(90 * time.Second)),
			want: 90 * time.Second,
		},
		{
			// срок операции может быть не задан: тогда его просто нечем назвать
			name: "нулевой срок доезжает как есть",
			err:  mrauth.ErrSignupAlreadyInProgressTryLater.Wrap(mrauth.NewRetryAfterError(0)),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var retryErr *mrauth.RetryAfterError

			require.True(t, errors.As(tt.err, &retryErr))
			assert.Equal(t, tt.want, retryErr.RetryAfter)
		})
	}
}

// TestRetryAfterErrorNotFoundInPlainSentinel - голый сентинел срока не несёт: контроллер
// не должен выдумывать заголовок там, где называть нечего.
func TestRetryAfterErrorNotFoundInPlainSentinel(t *testing.T) {
	t.Parallel()

	var retryErr *mrauth.RetryAfterError

	assert.False(t, errors.As(mrauth.ErrSignupAlreadyInProgressTryLater, &retryErr))
	assert.False(t, errors.As(mrauth.ErrSessionLimitExceededTryLater, &retryErr))
}
