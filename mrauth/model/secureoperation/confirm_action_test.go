package secureoperation_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// newOpWithSingleTOTPAction - создаёт операцию в статусе Opened с одним
// не-Sendable() действием method=TOTP.
func newOpWithSingleTOTPAction(t *testing.T) secureoperation.SecureOperation {
	t.Helper()

	op, err := secureoperation.NewOperation(
		"token",
		"name1",
		uuid.Nil,
		[]secureoperation.ConfirmAction{
			{
				Method:      confirmmethod.TOTP,
				MaxAttempts: 3,
				Expiry:      10 * time.Minute,
			},
		},
		nil,
	)
	require.NoError(t, err)

	return op
}

func TestConfirmAction_TOTPActionUsesCheckCode(t *testing.T) {
	t.Parallel()

	op := newOpWithSingleTOTPAction(t)

	confirmed, err := op.ConfirmAction(func(action secureoperation.ConfirmAction) (bool, error) {
		return action.Method == confirmmethod.TOTP, nil // имитация успешной внешней проверки
	})
	require.NoError(t, err)
	require.True(t, confirmed)
}

func TestConfirmAction_TOTPActionCheckCodeFails(t *testing.T) {
	t.Parallel()

	op := newOpWithSingleTOTPAction(t)
	before := op.RemainingAttempts

	confirmed, err := op.ConfirmAction(func(action secureoperation.ConfirmAction) (bool, error) {
		return false, nil // имитация неуспешной внешней проверки
	})
	require.ErrorIs(t, err, secureoperation.ErrConfirmCodeIsIncorrect)
	require.False(t, confirmed)
	require.Equal(t, before-1, op.RemainingAttempts)
}
