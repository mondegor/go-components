package unit_test

import (
	"testing"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/pendingoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

// confirmedOperation - подтверждённая операция указанного типа с указанным payload.
func confirmedOperation(t *testing.T, name string, payload []byte) secureoperation.SecureOperation {
	t.Helper()

	op := secureoperation.SecureOperation{
		Token:     "token-" + name,
		Name:      name,
		Payload:   payload,
		Status:    operationstatus.Confirmed,
		ExpiresAt: time.Now().UTC().Add(time.Hour).Round(time.Second),
	}
	require.NoError(t, secureoperation.WakeUp(&op, nil))

	return op
}

// TestNewPendingOperationTypes - операции личного кабинета сопоставлены типам контракта, у смены
// емаила (оба шага) и телефона разобраны новые адреса; вход и регистрация в список не входят.
func TestNewPendingOperationTypes(t *testing.T) {
	t.Parallel()

	emailPayload, err := unit.BuildChangeEmailPayload(dto.ChangeEmailOperation{NewEmail: "new@example.com", Email: "user@example.com"})
	require.NoError(t, err)

	phonePayload, err := unit.BuildChangePhonePayload(dto.ChangePhoneOperation{NewPhone: 79991234567, Email: "user@example.com"})
	require.NoError(t, err)

	type testCase struct {
		name         string
		payload      []byte
		wantOK       bool
		wantType     operationtype.Enum
		wantNewEmail string
		wantNewPhone uint64
	}

	tests := []testCase{
		{name: unit.NameAuthorizeUser},
		{name: unit.NameConfirmCreateUser},
		{name: unit.NameConfirmChangeEmailRequest, payload: emailPayload, wantOK: true, wantType: operationtype.ChangeEmail, wantNewEmail: "new@example.com"},
		{name: unit.NameConfirmChangeEmail, payload: emailPayload, wantOK: true, wantType: operationtype.ChangeEmailConfirm, wantNewEmail: "new@example.com"},
		{name: unit.NameConfirmChangePhone, payload: phonePayload, wantOK: true, wantType: operationtype.ChangePhone, wantNewPhone: 79991234567},
		{name: unit.NameConfirmChangePassword, wantOK: true, wantType: operationtype.ChangePassword},
		{name: unit.NameConfirmChangeTOTP, wantOK: true, wantType: operationtype.ChangeTOTP},
		{name: unit.NameConfirmRegenerateRecovery, wantOK: true, wantType: operationtype.RegenerateRecovery},
		{name: unit.NameConfirmDisable2FA, wantOK: true, wantType: operationtype.Disable2FA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			op := confirmedOperation(t, tt.name, tt.payload)

			item, ok, err := unit.NewPendingOperation(op)
			require.NoError(t, err)
			require.Equal(t, tt.wantOK, ok)

			if !tt.wantOK {
				return
			}

			assert.Equal(t, pendingoperation.PendingOperation{
				Token:     op.Token,
				Type:      tt.wantType,
				Status:    operationstatus.Confirmed,
				ExpiresAt: op.ExpiresAt,
				NewEmail:  tt.wantNewEmail,
				NewPhone:  tt.wantNewPhone,
			}, item)
		})
	}
}

// TestNewPendingOperationCurrentAction - звено отдаётся только у неподтверждённой операции
// (у подтверждённой проверено в TestNewPendingOperationTypes), а счётчики повторной отправки -
// только у звена, которое её допускает (код по емаилу, но не TOTP).
func TestNewPendingOperationCurrentAction(t *testing.T) {
	t.Parallel()

	resendsAt := time.Now().UTC().Add(time.Minute).Round(time.Second)
	expiresAt := time.Now().UTC().Add(time.Hour).Round(time.Second)

	t.Run("email action", func(t *testing.T) {
		t.Parallel()

		payload, err := unit.BuildChangeEmailPayload(dto.ChangeEmailOperation{NewEmail: "new@example.com", Email: "user@example.com"})
		require.NoError(t, err)

		op := secureoperation.SecureOperation{
			Token:             "token",
			Name:              unit.NameConfirmChangeEmail,
			Payload:           payload,
			RemainingAttempts: 3,
			RemainingResends:  2,
			ResendsAt:         resendsAt,
			Status:            operationstatus.Opened,
			ExpiresAt:         expiresAt,
		}
		require.NoError(t, secureoperation.WakeUp(&op, []secureoperation.ConfirmAction{
			{Method: confirmmethod.Email, MaxAttempts: 3, Expiry: time.Hour, Address: "new@example.com"},
		}))

		item, ok, err := unit.NewPendingOperation(op)
		require.NoError(t, err)
		require.True(t, ok)

		remainingResends := int16(2)
		assert.Equal(t, &pendingoperation.PendingAction{
			Method:            confirmmethod.Email,
			RemainingAttempts: 3,
			RemainingResends:  &remainingResends,
			ResendsAt:         &resendsAt,
		}, item.CurrentAction)
	})

	t.Run("totp action", func(t *testing.T) {
		t.Parallel()

		op := secureoperation.SecureOperation{
			Token:             "token",
			Name:              unit.NameConfirmDisable2FA,
			RemainingAttempts: 5,
			Status:            operationstatus.Opened,
			ExpiresAt:         expiresAt,
		}
		require.NoError(t, secureoperation.WakeUp(&op, []secureoperation.ConfirmAction{
			{Method: confirmmethod.TOTP, MaxAttempts: 5, Expiry: time.Hour},
		}))

		item, ok, err := unit.NewPendingOperation(op)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, &pendingoperation.PendingAction{Method: confirmmethod.TOTP, RemainingAttempts: 5}, item.CurrentAction)
	})
}

// TestNewPendingOperationBrokenPayload - нечитаемый payload - нарушение инварианта: ошибка,
// а не пропуск операции.
func TestNewPendingOperationBrokenPayload(t *testing.T) {
	t.Parallel()

	op := confirmedOperation(t, unit.NameConfirmChangeEmail, []byte(`{"new_email":""}`))

	_, ok, err := unit.NewPendingOperation(op)
	require.ErrorIs(t, err, errors.ErrInternalIncorrectInputData)
	assert.False(t, ok)
}
