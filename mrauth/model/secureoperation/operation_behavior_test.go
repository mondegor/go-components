package secureoperation_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth"
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
		Method:           confirmmethod.Email,
		MaxAttempts:      3,
		MaxResends:       5,
		MinResendTime:    5 * time.Minute,
		Expiry:           10 * time.Minute,
		Address:          address,
		ConfirmCode:      code,
		PlainConfirmCode: code,
	}
}

func totpAction() secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{Method: confirmmethod.TOTP, MaxAttempts: 3, Expiry: 10 * time.Minute}
}

func TestSecureOperation_Notify_SendableSendsCode(t *testing.T) {
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
}

func TestSecureOperation_Notify_NonSendableDoesNothing(t *testing.T) {
	t.Parallel()

	op := openedOp(t, totpAction())
	called := false

	require.NoError(t, op.Notify(func(confirmmethod.Enum, string, string) error {
		called = true

		return nil
	}))
	assert.False(t, called)
}

func TestSecureOperation_Notify_NilCallbackDoesNothing(t *testing.T) {
	t.Parallel()

	op := openedOp(t, emailAction("u@e", "code123"))
	require.NoError(t, op.Notify(nil))
}

func TestSecureOperation_Notify_EmptyAddressFails(t *testing.T) {
	t.Parallel()

	op := openedOp(t, emailAction("", "code123"))

	err := op.Notify(func(confirmmethod.Enum, string, string) error { return nil })
	require.ErrorContains(t, err, "address is empty")
}

func TestSecureOperation_Notify_EmptyCodeFails(t *testing.T) {
	t.Parallel()

	op := openedOp(t, emailAction("u@e", ""))

	err := op.Notify(func(confirmmethod.Enum, string, string) error { return nil })
	require.ErrorContains(t, err, "confirmCode is empty")
}

func TestSecureOperation_InitSendable_SetsGeneratedCode(t *testing.T) {
	t.Parallel()

	op := openedOp(t, emailAction("u@e", ""))

	require.NoError(t, op.InitSendableAction(func() (string, string, error) { return "newcode", "hashedcode", nil }))

	action, ok := op.FirstAction()
	require.True(t, ok)
	assert.Equal(t, "hashedcode", action.ConfirmCode)   // в хранилище идёт хеш
	assert.Equal(t, "newcode", action.PlainConfirmCode) // открытый код - только для отправки
}

func TestSecureOperation_InitSendable_NonSendableSkipped(t *testing.T) {
	t.Parallel()

	op := openedOp(t, totpAction())
	called := false

	require.NoError(t, op.InitSendableAction(func() (string, string, error) {
		called = true

		return "x", "x", nil
	}))
	assert.False(t, called)
}

func TestSecureOperation_InitSendable_NilGeneratorFails(t *testing.T) {
	t.Parallel()

	op := openedOp(t, emailAction("u@e", ""))
	require.ErrorIs(t, op.InitSendableAction(nil), sysmesserrors.ErrInternalNilPointer)
}

func TestSecureOperation_OpenedWithAction(t *testing.T) {
	t.Parallel()

	op := openedOp(t, totpAction())

	assert.True(t, op.Is(operationstatus.Opened))
	assert.False(t, op.Is(operationstatus.Confirmed))

	action, ok := op.FirstAction()
	require.True(t, ok)
	assert.Equal(t, confirmmethod.TOTP, action.Method)
	assert.Len(t, op.Actions(), 1)
}

func TestSecureOperation_ConfirmedWithoutActions(t *testing.T) {
	t.Parallel()

	op := wokenOp(t, totpAction(), operationstatus.Confirmed, time.Time{}, 0)

	_, ok := op.FirstAction()
	assert.False(t, ok)
	assert.Empty(t, op.Actions())
}

func TestSecureOperation_ActivateResendCode_Success(t *testing.T) {
	t.Parallel()

	op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Opened, time.Now().Add(-time.Minute), 5)

	require.NoError(t, op.ActivateResendCode("new-token"))
	assert.Equal(t, "new-token", op.Token)
	assert.Equal(t, int16(4), op.RemainingResends)
}

func TestSecureOperation_ActivateResendCode_EmptyTokenFails(t *testing.T) {
	t.Parallel()

	op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Opened, time.Now().Add(-time.Minute), 5)
	require.ErrorContains(t, op.ActivateResendCode(""), "token is empty")
}

func TestSecureOperation_ActivateResendCode_ConfirmedFails(t *testing.T) {
	t.Parallel()

	op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Confirmed, time.Time{}, 5)
	require.ErrorIs(t, op.ActivateResendCode("new-token"), mrauth.ErrOperationAlreadyConfirmed)
}

func TestSecureOperation_ActivateResendCode_NonSendableFails(t *testing.T) {
	t.Parallel()

	op := wokenOp(t, totpAction(), operationstatus.Opened, time.Now().Add(-time.Minute), 5)
	require.ErrorIs(t, op.ActivateResendCode("new-token"), mrauth.ErrResendCodeIsNotSupported)
}

func TestSecureOperation_ActivateResendCode_NoResendsLeftFails(t *testing.T) {
	t.Parallel()

	op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Opened, time.Now().Add(-time.Minute), 0)
	require.ErrorIs(t, op.ActivateResendCode("new-token"), mrauth.ErrNoAttemptsToResendCode)
}

func TestSecureOperation_ActivateResendCode_TooSoonRestricted(t *testing.T) {
	t.Parallel()

	op := wokenOp(t, emailAction("u@e", "c"), operationstatus.Opened, time.Now().Add(time.Minute), 5)
	require.ErrorIs(t, op.ActivateResendCode("new-token"), mrauth.ErrSendingNewMessagesIsTemporarilyRestricted)
}

func TestSecureOperation_ConfirmAction_FirstOfTwoKeepsOpened(t *testing.T) {
	t.Parallel()

	op, err := secureoperation.NewOperation(
		"token",
		"name1",
		uuid.New(),
		[]secureoperation.ConfirmAction{emailAction("u@e", "c"), totpAction()},
		nil,
	)
	require.NoError(t, err)

	confirmed, err := op.ConfirmAction(confirmOK)
	require.NoError(t, err)
	assert.False(t, confirmed)

	action, ok := op.FirstAction()
	require.True(t, ok)
	assert.Equal(t, confirmmethod.TOTP, action.Method)
}

func TestSecureOperation_ConfirmAction_AlreadyConfirmedFails(t *testing.T) {
	t.Parallel()

	op := wokenOp(t, totpAction(), operationstatus.Confirmed, time.Time{}, 0)

	confirmed, err := op.ConfirmAction(confirmOK)
	require.ErrorIs(t, err, mrauth.ErrOperationAlreadyConfirmed)
	assert.False(t, confirmed)
}

func TestSecureOperation_ConfirmAction_NoAttemptsFails(t *testing.T) {
	t.Parallel()

	op := wokenOp(t, secureoperation.ConfirmAction{Method: confirmmethod.TOTP, MaxAttempts: 0, Expiry: time.Minute}, operationstatus.Opened, time.Time{}, 0)

	confirmed, err := op.ConfirmAction(confirmOK)
	require.ErrorIs(t, err, mrauth.ErrNoAttemptsToConfirmOperation)
	assert.False(t, confirmed)
}

// confirmOK - проверка текущего действия, которая всегда успешна.
func confirmOK(secureoperation.ConfirmAction) (bool, error) {
	return true, nil
}

// emailOpWithExpiry - восстанавливает Opened-операцию с одним email-действием указанного
// срока жизни и заданным сроком действия операции; повторная отправка кода ей уже разрешена.
func emailOpWithExpiry(t *testing.T, expiry time.Duration, expiresAt time.Time) secureoperation.SecureOperation {
	t.Helper()

	action := emailAction("u@e", "c")
	action.Expiry = expiry

	op := secureoperation.SecureOperation{
		Token:             "token",
		Name:              "name1",
		UserID:            uuid.New(),
		RemainingAttempts: action.MaxAttempts,
		RemainingResends:  action.MaxResends,
		ResendsAt:         time.Now().Add(-time.Minute),
		Status:            operationstatus.Opened,
		ExpiresAt:         expiresAt,
	}
	require.NoError(t, secureoperation.WakeUp(&op, []secureoperation.ConfirmAction{action}))

	return op
}

// TestSecureOperation_FixedExpiry_SetOnCreate - фиксированный срок (больше порога)
// назначается при создании операции так же, как обычный: от момента создания.
func TestSecureOperation_FixedExpiry_SetOnCreate(t *testing.T) {
	t.Parallel()

	action := emailAction("u@e", "c")
	action.Expiry = 72 * time.Hour

	op := openedOp(t, action)

	assert.WithinDuration(t, time.Now().Add(72*time.Hour), op.ExpiresAt, 2*time.Second)
}

// TestSecureOperation_FixedExpiry_Renewal - срок действия больше порога модели фиксирован:
// повторная отправка кода и подтверждение его не продлевают. Срок не больше порога (включая
// ровно порог) на каждом из этих шагов отсчитывается заново.
func TestSecureOperation_FixedExpiry_Renewal(t *testing.T) {
	t.Parallel()

	const threshold = 30 * time.Minute // порог продления срока модели

	type testCase struct {
		name      string
		expiry    time.Duration
		wantFixed bool
	}

	tests := []testCase{
		{name: "above threshold is fixed", expiry: 72 * time.Hour, wantFixed: true},
		{name: "just above threshold is fixed", expiry: threshold + time.Second, wantFixed: true},
		{name: "exactly threshold is renewed", expiry: threshold, wantFixed: false},
		{name: "below threshold is renewed", expiry: 10 * time.Minute, wantFixed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// исходный срок заведомо отличается от "сейчас + expiry" в каждом из случаев
			deadline := time.Now().Add(5 * time.Minute).UTC().Round(time.Second)
			want := deadline

			if !tt.wantFixed {
				want = time.Now().Add(tt.expiry)
			}

			resent := emailOpWithExpiry(t, tt.expiry, deadline)
			require.NoError(t, resent.ActivateResendCode("new-token"))
			assert.Equal(t, "new-token", resent.Token)
			assert.WithinDuration(t, want, resent.ExpiresAt, 2*time.Second)

			confirmedOp := emailOpWithExpiry(t, tt.expiry, deadline)
			confirmed, err := confirmedOp.ConfirmAction(confirmOK)
			require.NoError(t, err)
			require.True(t, confirmed)
			assert.WithinDuration(t, want, confirmedOp.ExpiresAt, 2*time.Second)
		})
	}
}
