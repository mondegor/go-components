package httpv1_test

import (
	"testing"

	"github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// TestErrorCustomCodeFormat - закрепляет вид кода 400, который контроллеры отдают клиенту
// для ошибок, привязанных к полю запроса: "<БазовыйКод>/<поле>". Именно этот вид описан
// в контракте (Api.Response.Model.ErrorAttribute), поэтому смена разделителя или схемы
// склейки в go-core должна ронять тест, а не молча расходиться со спекой.
func TestErrorCustomCodeFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		fieldName string
		want      string
	}{
		{"signup: емаил занят", mrauth.ErrEmailAlreadyExists, "user_email", "EmailAlreadyExists/user_email"},
		{"check-login: логин занят", mrauth.ErrEmailAlreadyExists, "user_login", "EmailAlreadyExists/user_login"},
		{"signin: логина не существует", mrauth.ErrLoginNotExists, "user_login", "LoginNotExists/user_login"},
		{"смена емаила: емаил занят", mrauth.ErrEmailAlreadyExists, "new_email", "EmailAlreadyExists/new_email"},
		{"смена телефона: телефон занят", mrauth.ErrPhoneAlreadyExists, "new_phone", "PhoneAlreadyExists/new_phone"},
		{"привязка TOTP: код не совпал", mrauth.ErrTOTPCodeIsIncorrect, "totp_code", "TOTPCodeIsIncorrect/totp_code"},
		{"токен операции недействителен", secureoperation.ErrOperationInvalid, "token", "OperationInvalid/token"},
		{"повторные отправки исчерпаны", secureoperation.ErrNoAttemptsToResendCode, "token", "NoAttemptsToResendCode/token"},
		{
			"повторная отправка неприменима к текущему действию",
			secureoperation.ErrResendCodeIsNotSupported,
			"token",
			"ResendCodeIsNotSupported/token",
		},
		{"идентификаторы сессий", mrauth.ErrSessionIDIsInvalid, "session_ids", "SessionIDIsInvalid/session_ids"},
		{"операция не подтверждена", secureoperation.ErrOperationIsNotConfirmed, "token", "OperationIsNotConfirmed/token"},
		{"операция истекла", secureoperation.ErrOperationAlreadyExpired, "token", "OperationAlreadyExpired/token"},
		{"операция уже подтверждена", secureoperation.ErrOperationAlreadyConfirmed, "token", "OperationAlreadyConfirmed/token"},
		{"код подтверждения не передан", secureoperation.ErrConfirmCodeIsRequired, "secret", "ConfirmCodeIsRequired/secret"},
		{"код подтверждения неверен", secureoperation.ErrConfirmCodeIsIncorrect, "secret", "ConfirmCodeIsIncorrect/secret"},
		{"попытки подтверждения исчерпаны", secureoperation.ErrNoAttemptsToConfirmOperation, "secret", "NoAttemptsToConfirmOperation/secret"},
		{
			"повторная отправка ограничена",
			secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted,
			"token",
			"SendingNewMessagesIsTemporarilyRestricted/token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			customErr := errors.WithCustomCode(tt.err, tt.fieldName)

			require.True(t, customErr.IsKindUser(), "ошибка должна остаться пользовательской, иначе ответ будет не 400")
			assert.Equal(t, tt.want, customErr.CustomCode())
		})
	}
}

// TestErrorCustomCodeKeepsSentinel - привязка к полю не должна ломать errors.Is:
// на неё опираются проверки семейства ошибок в контроллерах.
func TestErrorCustomCodeKeepsSentinel(t *testing.T) {
	t.Parallel()

	customErr := errors.WithCustomCode(secureoperation.ErrOperationInvalid, "token")

	require.ErrorIs(t, customErr, secureoperation.ErrOperationInvalid)
	require.NotErrorIs(t, customErr, secureoperation.ErrOperationIsNotConfirmed)
}
