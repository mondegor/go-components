package bag_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-webcore/mrserver/mrresp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// TestOperationResponse_Resends - счётчики повторных отправок отдаются только когда они
// применимы (емаил и телефон), но ноль в них - значение, а не отсутствие: он доезжает
// до клиента и означает исчерпанные отправки и разрешённую прямо сейчас отправку.
func TestOperationResponse_Resends(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                 string
		action               secureoperation.ConfirmAction
		resendsAt            time.Time
		remainingResends     int16
		wantRemainingResends *int16
		wantResendsIn        *int64
	}

	tests := []testCase{
		{
			// время повторной отправки уже наступило: 0 секунд - это значение,
			// а не повод убрать поле из ответа
			name:                 "email, resend is allowed right now",
			action:               emailAction(),
			resendsAt:            time.Now().UTC().Add(-time.Minute),
			remainingResends:     2,
			wantRemainingResends: ptr(int16(2)),
			wantResendsIn:        ptr(int64(0)),
		},
		{
			name:                 "email, resend is delayed",
			action:               emailAction(),
			resendsAt:            time.Now().UTC().Add(2 * time.Minute),
			remainingResends:     1,
			wantRemainingResends: ptr(int16(1)),
			wantResendsIn:        ptr(int64(120)),
		},
		{
			// отправки исчерпаны: поле остаётся в ответе с нулём
			name:                 "email, resends are exhausted",
			action:               emailAction(),
			resendsAt:            time.Now().UTC().Add(-time.Minute),
			remainingResends:     0,
			wantRemainingResends: ptr(int16(0)),
			wantResendsIn:        ptr(int64(0)),
		},
		{
			name:      "totp, resends are not applicable",
			action:    totpAction(),
			resendsAt: time.Time{},
		},
		{
			name:      "password, resends are not applicable",
			action:    passwordAction(),
			resendsAt: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			op := wokenOp(t, tt.action, tt.resendsAt, tt.remainingResends)
			response := bag.NewOperationResponse(nil)

			waiting := response.NewConfirmOperation(op, "message")
			assert.Equal(t, tt.wantRemainingResends, waiting.RemainingResends)
			assert.Equal(t, tt.wantResendsIn, waiting.ResendsIn)

			state := response.NewErrorConfirmOperation(mrresp.Error400Response{}, op).OperationState
			assert.Equal(t, tt.wantRemainingResends, state.RemainingResends)
			assert.Equal(t, tt.wantResendsIn, state.ResendsIn)
		})
	}
}

// TestOperationResponse_ResendsWithoutActions - у операции без действий метод подтверждения
// не заполнен, поэтому счётчики повторных отправок не применимы и не отдаются, даже если
// в самой операции они заполнены.
func TestOperationResponse_ResendsWithoutActions(t *testing.T) {
	t.Parallel()

	op := secureoperation.SecureOperation{
		Token:            "token",
		Status:           operationstatus.Opened,
		RemainingResends: 3,
		ResendsAt:        time.Now().UTC().Add(2 * time.Minute),
		ExpiresAt:        time.Now().UTC().Add(10 * time.Minute),
	}
	response := bag.NewOperationResponse(nil)

	waiting := response.NewConfirmOperation(op, "message")
	assert.Nil(t, waiting.RemainingResends)
	assert.Nil(t, waiting.ResendsIn)

	state := response.NewErrorConfirmOperation(mrresp.Error400Response{}, op).OperationState
	assert.Nil(t, state.RemainingResends)
	assert.Nil(t, state.ResendsIn)
}

// TestOperationResponse_ResendsJSON - проверяется именно то, что видит клиент: omitempty
// на указателе снимает поле только при nil, поэтому ноль из ответа не исчезает,
// а неприменимые счётчики в нём не появляются.
func TestOperationResponse_ResendsJSON(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		action secureoperation.ConfirmAction
		want   string
	}

	tests := []testCase{
		{
			name:   "email, zero is sent",
			action: emailAction(),
			want:   `"remaining_resends":0,"resends_in":0`,
		},
		{
			name:   "totp, fields are absent",
			action: totpAction(),
			want:   `"remaining_attempts":3,"expires_in":`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			op := wokenOp(t, tt.action, time.Now().UTC().Add(-time.Minute), 0)

			got, err := json.Marshal(bag.NewOperationResponse(nil).NewConfirmOperation(op, ""))
			require.NoError(t, err)

			assert.Contains(t, string(got), tt.want)
		})
	}
}

// wokenOp - восстанавливает операцию с явно заданными счётчиками повторных отправок.
func wokenOp(t *testing.T, action secureoperation.ConfirmAction, resendsAt time.Time, remainingResends int16) secureoperation.SecureOperation {
	t.Helper()

	op := secureoperation.SecureOperation{
		Token:             "token",
		Name:              "name1",
		UserID:            uuid.New(),
		RemainingAttempts: action.MaxAttempts,
		RemainingResends:  remainingResends,
		ResendsAt:         resendsAt,
		Status:            operationstatus.Opened,
		ExpiresAt:         time.Now().UTC().Add(10 * time.Minute),
	}
	require.NoError(t, secureoperation.WakeUp(&op, []secureoperation.ConfirmAction{action}))

	return op
}

func emailAction() secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{
		Method:        confirmmethod.Email,
		MaxAttempts:   3,
		MaxResends:    5,
		MinResendTime: 5 * time.Minute,
		Expiry:        10 * time.Minute,
		Address:       "user@example.com",
	}
}

func totpAction() secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{Method: confirmmethod.TOTP, MaxAttempts: 3, Expiry: 10 * time.Minute}
}

func passwordAction() secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{Method: confirmmethod.Password, MaxAttempts: 3, Expiry: 10 * time.Minute}
}

// ptr - возвращает указатель на значение: счётчики повторных отправок - указатели,
// чтобы отличать неприменимость от нуля.
func ptr[T any](value T) *T {
	return &value
}
