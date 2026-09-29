package secureoperation_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

func Test_NewOperationWithError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		token          string
		operationType  operationtype.Enum
		actions        []secureoperation.ConfirmAction
		wantErrMessage string
	}{
		{
			name:           "test1",
			wantErrMessage: "type is empty",
		},
		{
			name:           "test2",
			operationType:  operationtype.ChangePhone,
			wantErrMessage: "operation is opened, but len(actions) == 0",
		},
		{
			name:          "test3",
			operationType: operationtype.ChangePhone,
			actions: []secureoperation.ConfirmAction{
				{
					Method: 0,
				},
			},
			wantErrMessage: "action without method",
		},
		{
			name:          "test4",
			operationType: operationtype.ChangePhone,
			actions: []secureoperation.ConfirmAction{
				{
					Method: confirmmethod.Email,
				},
			},
			wantErrMessage: "token is empty",
		},
		{
			// код подтверждения генерится при переходе к следующему звену, поэтому
			// sendable-звено после не-sendable недостижимо
			name:          "sendable action after non-sendable",
			operationType: operationtype.ChangePhone,
			actions: []secureoperation.ConfirmAction{
				{Method: confirmmethod.TOTP},
				{Method: confirmmethod.Email},
			},
			wantErrMessage: "sendable action must precede non-sendable",
		},
		{
			// иначе аварийный код гасился бы до того, как комбинация принята целиком
			name:          "allow recovery on non-last action",
			operationType: operationtype.ChangePhone,
			actions: []secureoperation.ConfirmAction{
				{Method: confirmmethod.Email, AllowRecovery: true},
				{Method: confirmmethod.TOTP},
			},
			wantErrMessage: "recovery code is accepted by the last action only",
		},
		{
			name:          "recovery action is not the last",
			operationType: operationtype.ChangePhone,
			actions: []secureoperation.ConfirmAction{
				{Method: confirmmethod.Recovery},
				{Method: confirmmethod.TOTP},
			},
			wantErrMessage: "recovery code is accepted by the last action only",
		},
		{
			// звено последнее, поэтому правило "только последним" его пропускает, а вот код
			// такого звена сверяет сама операция, а не верификатор: признак там ничего не значит
			// и молча маскировал бы неверно собранную цепочку
			name:          "allow recovery on sendable action",
			operationType: operationtype.ChangePhone,
			actions: []secureoperation.ConfirmAction{
				{Method: confirmmethod.Email, AllowRecovery: true},
			},
			wantErrMessage: "sendable action cannot allow recovery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := secureoperation.NewOperation(tt.token, tt.operationType, uuid.Nil, tt.actions, nil)
			assert.ErrorContains(t, err, tt.wantErrMessage)
		})
	}
}

// Test_NewOperationRecoveryChain - цепочка "второй фактор -> аварийный код" состоит из двух
// не-sendable звеньев и потому легальна: правило порядка запрещает только sendable после
// не-sendable, а аварийный код стоит последним.
func Test_NewOperationRecoveryChain(t *testing.T) {
	t.Parallel()

	op, err := secureoperation.NewOperation(
		"token",
		operationtype.ChangePhone,
		uuid.Nil,
		[]secureoperation.ConfirmAction{
			{Method: confirmmethod.TOTP, MaxAttempts: 3, Expiry: time.Minute},
			{Method: confirmmethod.Recovery, MaxAttempts: 3, Expiry: time.Minute},
		},
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, confirmmethod.TOTP, op.FirstActionMethod())

	// письмо по такой цепочке не отправляется: у первого звена нечего отправлять
	assert.NoError(t, op.NotifyByEmail(func(_, _ string) error {
		t.Fatal("sendCodeFunc must not be called for a non-sendable action")

		return nil
	}))
}

func Test_WakeUpOperationWithError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		token          string
		operationType  operationtype.Enum
		actions        []secureoperation.ConfirmAction
		status         operationstatus.Enum
		wantErrMessage string
	}{
		{
			name:           "test1",
			wantErrMessage: "token is empty",
		},
		{
			name:           "test2",
			token:          "token",
			wantErrMessage: "type is empty",
		},
		{
			name:           "test3",
			token:          "token",
			operationType:  operationtype.ChangePhone,
			wantErrMessage: "operation status is unknown",
		},
		{
			name:          "test4",
			token:         "token",
			operationType: operationtype.ChangePhone,
			actions: []secureoperation.ConfirmAction{
				{
					Method: 0,
				},
			},
			status:         operationstatus.Opened,
			wantErrMessage: "action without method",
		},
		{
			name:          "test5",
			token:         "token",
			operationType: operationtype.ChangePhone,
			actions: []secureoperation.ConfirmAction{
				{},
			},
			status:         operationstatus.Confirmed,
			wantErrMessage: "operation is confirmed, but len(actions) > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			op := secureoperation.SecureOperation{
				Token:             tt.token,
				Type:              tt.operationType,
				UserID:            uuid.Nil,
				RemainingAttempts: 0,
				RemainingResends:  0,
				ResendsAt:         time.Time{},
				Payload:           nil,
				Status:            tt.status,
				ExpiresAt:         time.Time{},
			}

			assert.ErrorContains(t, secureoperation.WakeUp(&op, tt.actions), tt.wantErrMessage)
		})
	}
}
