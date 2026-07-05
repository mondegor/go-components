package secureoperation_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	secureoperation_model "github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// openedEmailOp - создаёт sendable-операцию (Email) в статусе Opened, готовую к
// повторной отправке кода (есть оставшиеся попытки, ResendsAt в прошлом).
func openedEmailOp(t *testing.T) secureoperation_model.SecureOperation {
	t.Helper()

	op := secureoperation_model.SecureOperation{
		Token:             "token",
		Name:              "name1",
		UserID:            uuid.New(),
		RemainingAttempts: 3,
		RemainingResends:  5,
		ResendsAt:         time.Now().Add(-time.Minute),
		Status:            operationstatus.Opened,
		ExpiresAt:         time.Now().Add(10 * time.Minute),
	}
	require.NoError(t, secureoperation_model.WakeUp(&op, []secureoperation_model.ConfirmAction{
		{
			Method:        confirmmethod.Email,
			MaxAttempts:   3,
			MaxResends:    5,
			MinResendTime: 5 * time.Minute,
			Expiry:        10 * time.Minute,
			Address:       "u@e",
		},
	}))

	return op
}

func TestResendCode_Prepare_Success(t *testing.T) {
	t.Parallel()

	resend := secureoperation.NewResendCode(
		&fakeTokenGen{token: "new-token"},
		&fakeCodeGen{code: "123456"},
	)

	out, err := resend.Prepare(openedEmailOp(t))
	require.NoError(t, err)
	require.Equal(t, "new-token", out.Token)

	action, ok := out.FirstAction()
	require.True(t, ok)
	require.Equal(t, "123456", action.ConfirmCode)
}

func TestResendCode_Prepare_TokenGeneratorError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("token generation failed")
	resend := secureoperation.NewResendCode(
		&fakeTokenGen{err: wantErr},
		&fakeCodeGen{code: "123456"},
	)

	_, err := resend.Prepare(openedEmailOp(t))
	require.ErrorIs(t, err, wantErr)
}

func TestResendCode_Prepare_NotOpenedFails(t *testing.T) {
	t.Parallel()

	confirmed := secureoperation_model.SecureOperation{
		Token:     "token",
		Name:      "name1",
		UserID:    uuid.New(),
		Status:    operationstatus.Confirmed,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	require.NoError(t, secureoperation_model.WakeUp(&confirmed, nil))

	resend := secureoperation.NewResendCode(
		&fakeTokenGen{token: "new-token"},
		&fakeCodeGen{code: "123456"},
	)

	_, err := resend.Prepare(confirmed)
	require.ErrorIs(t, err, secureoperation_model.ErrOperationAlreadyConfirmed)
}
