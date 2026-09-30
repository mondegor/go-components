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
func confirmedOperation(t *testing.T, opType operationtype.Enum, payload []byte) secureoperation.SecureOperation {
	t.Helper()

	op := secureoperation.SecureOperation{
		Token:     "token-" + opType.String(),
		Type:      opType,
		Payload:   payload,
		Status:    operationstatus.Confirmed,
		ExpiresAt: time.Now().UTC().Add(time.Hour).Round(time.Second),
	}
	require.NoError(t, secureoperation.WakeUp(&op, nil))

	return op
}

// TestNewPendingOperationTypes - в список входит только шаг 2 смены емаила с разобранным новым
// адресом; остальные операции, вход и регистрация в список не входят.
func TestNewPendingOperationTypes(t *testing.T) {
	t.Parallel()

	emailPayload, err := unit.BuildChangeEmailPayload(dto.ChangeEmailOperation{NewEmail: "new@example.com", Email: "user@example.com"})
	require.NoError(t, err)

	type testCase struct {
		opType       operationtype.Enum
		payload      []byte
		wantOK       bool
		wantNewEmail string
	}

	tests := []testCase{
		{opType: operationtype.AuthorizeUser},
		{opType: operationtype.CreateUser},
		{opType: operationtype.ChangeEmail, payload: emailPayload},
		{opType: operationtype.ChangeEmailConfirm, payload: emailPayload, wantOK: true, wantNewEmail: "new@example.com"},
		{opType: operationtype.ChangePhone},
		{opType: operationtype.ChangePassword},
		{opType: operationtype.ChangeTOTP},
		{opType: operationtype.RegenerateRecovery},
		{opType: operationtype.Disable2FA},
	}

	// типы для отбора в хранилище совпадают с входящими в список
	wantTypes := make([]operationtype.Enum, 0, len(tests))

	for _, tt := range tests {
		if tt.wantOK {
			wantTypes = append(wantTypes, tt.opType)
		}
	}

	assert.ElementsMatch(t, wantTypes, unit.PendingOperationTypes())

	for _, tt := range tests {
		t.Run(tt.opType.String(), func(t *testing.T) {
			t.Parallel()

			op := confirmedOperation(t, tt.opType, tt.payload)

			item, ok, err := unit.NewPendingOperation(op)
			require.NoError(t, err)
			require.Equal(t, tt.wantOK, ok)

			if !tt.wantOK {
				return
			}

			assert.Equal(t, dto.PendingOperation{
				Token:     op.Token,
				Type:      tt.opType,
				Status:    operationstatus.Confirmed,
				ExpiresAt: op.ExpiresAt,
				NewEmail:  tt.wantNewEmail,
			}, item)
		})
	}
}

// TestNewPendingOperationCurrentAction - звено отдаётся только у неподтверждённой операции
// (у подтверждённой проверено в TestNewPendingOperationTypes), а счётчики повторной отправки -
// только у звена, которое её допускает. В списке один тип операции, и его цепочка состоит из звена
// EMAIL, поэтому звено без повторной отправки (TOTP) подставлено в неё искусственно.
func TestNewPendingOperationCurrentAction(t *testing.T) {
	t.Parallel()

	resendsAt := time.Now().UTC().Add(time.Minute).Round(time.Second)
	remainingResends := int16(2)

	type testCase struct {
		name   string
		action secureoperation.ConfirmAction
		want   *dto.PendingAction
	}

	tests := []testCase{
		{
			name:   "sendable action",
			action: secureoperation.ConfirmAction{Method: confirmmethod.Email, MaxAttempts: 3, CodeLength: 6, Expiry: time.Hour, Address: "new@example.com"},
			want: &dto.PendingAction{
				Method:            confirmmethod.Email,
				RemainingAttempts: 3,
				RemainingResends:  &remainingResends,
				ResendsAt:         &resendsAt,
			},
		},
		{
			name:   "not sendable action",
			action: secureoperation.ConfirmAction{Method: confirmmethod.TOTP, MaxAttempts: 3, Expiry: time.Hour},
			want:   &dto.PendingAction{Method: confirmmethod.TOTP, RemainingAttempts: 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := unit.BuildChangeEmailPayload(dto.ChangeEmailOperation{NewEmail: "new@example.com", Email: "user@example.com"})
			require.NoError(t, err)

			op := secureoperation.SecureOperation{
				Token:             "token",
				Type:              operationtype.ChangeEmailConfirm,
				Payload:           payload,
				RemainingAttempts: 3,
				RemainingResends:  2,
				ResendsAt:         resendsAt,
				Status:            operationstatus.Opened,
				ExpiresAt:         time.Now().UTC().Add(time.Hour).Round(time.Second),
			}
			require.NoError(t, secureoperation.WakeUp(&op, []secureoperation.ConfirmAction{tt.action}))

			item, ok, err := unit.NewPendingOperation(op)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, tt.want, item.CurrentAction)
		})
	}
}

// TestNewPendingOperationBrokenPayload - нечитаемый payload - нарушение инварианта: ошибка,
// а не пропуск операции.
func TestNewPendingOperationBrokenPayload(t *testing.T) {
	t.Parallel()

	op := confirmedOperation(t, operationtype.ChangeEmailConfirm, []byte(`{"new_email":""}`))

	_, ok, err := unit.NewPendingOperation(op)
	require.ErrorIs(t, err, errors.ErrInternalIncorrectInputData)
	assert.False(t, ok)
}
