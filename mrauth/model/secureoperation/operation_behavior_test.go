package secureoperation_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// openedOp - создаёт операцию в статусе Opened с одним переданным действием.
func openedOp(t *testing.T, action secureoperation.ConfirmAction) secureoperation.SecureOperation {
	t.Helper()

	op, err := secureoperation.NewOperation("token", "name1", uuid.New(), []secureoperation.ConfirmAction{action}, nil)
	require.NoError(t, err)

	return op
}

// wokenOp - восстанавливает операцию через WakeUp с явно заданными полями
// (для контроля статуса, счётчиков и сроков).
func wokenOp(
	t *testing.T,
	action secureoperation.ConfirmAction,
	status operationstatus.Enum,
	resendsAt time.Time,
	remainingResends int16,
) secureoperation.SecureOperation {
	t.Helper()

	actions := []secureoperation.ConfirmAction{action}
	if status == operationstatus.Confirmed {
		actions = nil
	}

	op := secureoperation.SecureOperation{
		Token:             "token",
		Name:              "name1",
		UserID:            uuid.New(),
		RemainingAttempts: action.MaxAttempts,
		RemainingResends:  remainingResends,
		ResendsAt:         resendsAt,
		Payload:           []byte(`{"k":"v"}`),
		Status:            status,
		ExpiresAt:         time.Now().Add(10 * time.Minute),
	}
	require.NoError(t, secureoperation.WakeUp(&op, actions))

	return op
}

func emailAction(address, code string) secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{
		Method:        confirmmethod.Email,
		MaxAttempts:   3,
		MaxResends:    5,
		MinResendTime: 5 * time.Minute,
		Expiry:        10 * time.Minute,
		Address:       address,
		ConfirmCode:   code,
	}
}

func totpAction() secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{Method: confirmmethod.TOTP, MaxAttempts: 3, Expiry: 10 * time.Minute}
}

func TestSecureOperation_Notify(t *testing.T) {
	t.Parallel()

	t.Run("sendable action sends code", func(t *testing.T) {
		t.Parallel()

		op := openedOp(t, emailAction("u@e", "code123"))

		var gotMethod confirmmethod.Enum

		var gotAddress, gotCode string

		err := op.Notify(func(method confirmmethod.Enum, address, confirmCode string) error {
			gotMethod, gotAddress, gotCode = method, address, confirmCode

			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, confirmmethod.Email, gotMethod)
		assert.Equal(t, "u@e", gotAddress)
		assert.Equal(t, "code123", gotCode)
	})

	t.Run("non-sendable action does nothing", func(t *testing.T) {
		t.Parallel()

		op := openedOp(t, totpAction())
		called := false

		require.NoError(t, op.Notify(func(confirmmethod.Enum, string, string) error {
			called = true

			return nil
		}))
		assert.False(t, called)
	})

	t.Run("nil callback does nothing", func(t *testing.T) {
		t.Parallel()

		op := openedOp(t, emailAction("u@e", "code123"))
		require.NoError(t, op.Notify(nil))
	})

	t.Run("empty address fails", func(t *testing.T) {
		t.Parallel()

		op := openedOp(t, emailAction("", "code123"))

		err := op.Notify(func(confirmmethod.Enum, string, string) error { return nil })
		require.ErrorContains(t, err, "address is empty")
	})

	t.Run("empty code fails", func(t *testing.T) {
		t.Parallel()

		op := openedOp(t, emailAction("u@e", ""))

		err := op.Notify(func(confirmmethod.Enum, string, string) error { return nil })
		require.ErrorContains(t, err, "confirmCode is empty")
	})
}

func TestSecureOperation_InitSendableAction(t *testing.T) {
	t.Parallel()

	t.Run("sendable action sets generated code", func(t *testing.T) {
		t.Parallel()

		op := openedOp(t, emailAction("u@e", ""))

		require.NoError(t, op.InitSendableAction(func() (string, error) { return "newcode", nil }))

		action, ok := op.FirstAction()
		require.True(t, ok)
		assert.Equal(t, "newcode", action.ConfirmCode)
	})

	t.Run("non-sendable action is skipped", func(t *testing.T) {
		t.Parallel()

		op := openedOp(t, totpAction())
		called := false

		require.NoError(t, op.InitSendableAction(func() (string, error) {
			called = true

			return "x", nil
		}))
		assert.False(t, called)
	})

	t.Run("nil generator fails for sendable action", func(t *testing.T) {
		t.Parallel()

		op := openedOp(t, emailAction("u@e", ""))
		require.ErrorContains(t, op.InitSendableAction(nil), "generateCode is nil")
	})
}

func TestSecureOperation_UserInfo(t *testing.T) {
	t.Parallel()

	t.Run("confirmed returns payload", func(t *testing.T) {
		t.Parallel()

		op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Confirmed, time.Time{}, 0)

		info := op.UserInfo()
		assert.Equal(t, op.Token, info.Token)
		assert.Equal(t, op.UserID, info.UserID)
		assert.JSONEq(t, `{"k":"v"}`, string(info.Payload))
	})

	t.Run("opened returns empty", func(t *testing.T) {
		t.Parallel()

		op := openedOp(t, emailAction("u@e", "c"))
		assert.Equal(t, secureoperation.UserDTO{}, op.UserInfo())
	})
}

func TestSecureOperation_IsFirstActionActions(t *testing.T) {
	t.Parallel()

	t.Run("opened with action", func(t *testing.T) {
		t.Parallel()

		op := openedOp(t, totpAction())

		assert.True(t, op.Is(operationstatus.Opened))
		assert.False(t, op.Is(operationstatus.Confirmed))

		action, ok := op.FirstAction()
		require.True(t, ok)
		assert.Equal(t, confirmmethod.TOTP, action.Method)
		assert.Len(t, op.Actions(), 1)
	})

	t.Run("confirmed without actions", func(t *testing.T) {
		t.Parallel()

		op := wokenOp(t, totpAction(), operationstatus.Confirmed, time.Time{}, 0)

		_, ok := op.FirstAction()
		assert.False(t, ok)
		assert.Empty(t, op.Actions())
	})
}

func TestSecureOperation_ActivateResendCode(t *testing.T) {
	t.Parallel()

	past := time.Now().Add(-time.Minute)
	future := time.Now().Add(time.Minute)

	t.Run("success decrements resends and sets token", func(t *testing.T) {
		t.Parallel()

		op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Opened, past, 5)

		require.NoError(t, op.ActivateResendCode("new-token"))
		assert.Equal(t, "new-token", op.Token)
		assert.Equal(t, int16(4), op.RemainingResends)
	})

	t.Run("empty token fails", func(t *testing.T) {
		t.Parallel()

		op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Opened, past, 5)
		require.ErrorContains(t, op.ActivateResendCode(""), "token is empty")
	})

	t.Run("confirmed operation fails", func(t *testing.T) {
		t.Parallel()

		op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Confirmed, time.Time{}, 5)
		require.ErrorIs(t, op.ActivateResendCode("new-token"), secureoperation.ErrOperationAlreadyConfirmed)
	})

	t.Run("non-sendable action fails", func(t *testing.T) {
		t.Parallel()

		op := wokenOp(t, totpAction(), operationstatus.Opened, past, 5)
		require.ErrorContains(t, op.ActivateResendCode("new-token"), "action not support resend")
	})

	t.Run("no resends left fails", func(t *testing.T) {
		t.Parallel()

		op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Opened, past, 0)
		require.ErrorContains(t, op.ActivateResendCode("new-token"), "operation failed resends")
	})

	t.Run("too soon is restricted", func(t *testing.T) {
		t.Parallel()

		op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Opened, future, 5)
		require.ErrorIs(t, op.ActivateResendCode("new-token"), secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted)
	})
}

func TestSecureOperation_ConfirmActionFlow(t *testing.T) {
	t.Parallel()

	t.Run("first of two actions confirmed keeps operation opened", func(t *testing.T) {
		t.Parallel()

		op, err := secureoperation.NewOperation(
			"token",
			"name1",
			uuid.New(),
			[]secureoperation.ConfirmAction{emailAction("u@e", "c"), totpAction()},
			nil,
		)
		require.NoError(t, err)

		confirmed, err := op.ConfirmAction(func(secureoperation.ConfirmAction) (bool, error) { return true, nil })
		require.NoError(t, err)
		assert.False(t, confirmed)

		action, ok := op.FirstAction()
		require.True(t, ok)
		assert.Equal(t, confirmmethod.TOTP, action.Method)
	})

	t.Run("already confirmed fails", func(t *testing.T) {
		t.Parallel()

		op := wokenOp(t, totpAction(), operationstatus.Confirmed, time.Time{}, 0)

		confirmed, err := op.ConfirmAction(func(secureoperation.ConfirmAction) (bool, error) { return true, nil })
		require.ErrorIs(t, err, secureoperation.ErrOperationAlreadyConfirmed)
		assert.False(t, confirmed)
	})

	t.Run("no attempts left fails", func(t *testing.T) {
		t.Parallel()

		op := wokenOp(t, secureoperation.ConfirmAction{Method: confirmmethod.TOTP, MaxAttempts: 0, Expiry: time.Minute}, operationstatus.Opened, time.Time{}, 0)

		confirmed, err := op.ConfirmAction(func(secureoperation.ConfirmAction) (bool, error) { return true, nil })
		require.ErrorIs(t, err, secureoperation.ErrNoAttemptsToConfirmOperation)
		assert.False(t, confirmed)
	})
}
