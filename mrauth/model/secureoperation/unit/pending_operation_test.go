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

// TestNewPendingOperationTypes - в список входят только операции, в которых может быть применён
// аварийный код, и долгоживущие операции, у смены емаила (оба шага) разобран новый адрес;
// короткоживущие операции без аварийного кода, вход и регистрация в список не входят.
func TestNewPendingOperationTypes(t *testing.T) {
	t.Parallel()

	emailPayload, err := unit.BuildChangeEmailPayload(dto.ChangeEmailOperation{NewEmail: "new@example.com", Email: "user@example.com"})
	require.NoError(t, err)

	type testCase struct {
		name         string
		payload      []byte
		wantOK       bool
		wantType     operationtype.Enum
		wantNewEmail string
	}

	tests := []testCase{
		{name: unit.NameAuthorizeUser},
		{name: unit.NameConfirmCreateUser},
		{name: unit.NameConfirmChangeEmailRequest, payload: emailPayload, wantOK: true, wantType: operationtype.ChangeEmail, wantNewEmail: "new@example.com"},
		{name: unit.NameConfirmChangeEmail, payload: emailPayload, wantOK: true, wantType: operationtype.ChangeEmailConfirm, wantNewEmail: "new@example.com"},
		{name: unit.NameConfirmChangePhone},
		{name: unit.NameConfirmChangePassword},
		{name: unit.NameConfirmChangeTOTP},
		{name: unit.NameConfirmRegenerateRecovery},
		{name: unit.NameConfirmDisable2FA, wantOK: true, wantType: operationtype.Disable2FA},
	}

	// имена для отбора в хранилище совпадают с входящими в список
	wantNames := make([]string, 0, len(tests))

	for _, tt := range tests {
		if tt.wantOK {
			wantNames = append(wantNames, tt.name)
		}
	}

	assert.ElementsMatch(t, wantNames, unit.PendingOperationNames())

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

			assert.Equal(t, dto.PendingOperation{
				Token:     op.Token,
				Type:      tt.wantType,
				Status:    operationstatus.Confirmed,
				ExpiresAt: op.ExpiresAt,
				NewEmail:  tt.wantNewEmail,
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
		assert.Equal(t, &dto.PendingAction{
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
		assert.Equal(t, &dto.PendingAction{Method: confirmmethod.TOTP, RemainingAttempts: 5}, item.CurrentAction)
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
