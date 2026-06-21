package secureoperation_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	secureoperation_model "github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	fakeTokenGen struct {
		token string
		err   error
	}

	fakeCodeGen struct {
		code string
		err  error
	}

	fakeVerifier struct {
		ok           bool
		consume      func(ctx context.Context) error
		err          error
		calledMethod confirmmethod.Enum
		calledUserID uuid.UUID
	}
)

func (g *fakeTokenGen) GenToken() (string, error) {
	return g.token, g.err
}

func (g *fakeTokenGen) GenTokenLen(_ int) (string, error) {
	return g.token, g.err
}

func (g *fakeCodeGen) GenCode() (string, error) {
	return g.code, g.err
}

func (g *fakeCodeGen) GenCodeLen(_ int) (string, error) {
	return g.code, g.err
}

func (g *fakeCodeGen) HashedSecret(code string) (string, error) {
	return code, nil
}

func (g *fakeCodeGen) CompareSecretAndHash(_, _ string) error {
	return nil
}

func (v *fakeVerifier) Verify(
	_ context.Context,
	userID uuid.UUID,
	method confirmmethod.Enum,
	_ string,
) (bool, func(ctx context.Context) error, error) {
	v.calledMethod = method
	v.calledUserID = userID

	return v.ok, v.consume, v.err
}

// newOpWithSingleTOTPAction - создаёт операцию в статусе Opened с одним
// действием method=TOTP для указанного пользователя.
func newOpWithSingleTOTPAction(t *testing.T, userID uuid.UUID) secureoperation_model.SecureOperation {
	t.Helper()

	op, err := secureoperation_model.NewOperation(
		"token",
		"name1",
		userID,
		[]secureoperation_model.ConfirmAction{
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

func TestConfirmCode_TOTPVerifiedNoConsume(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	verifier := &fakeVerifier{ok: true, consume: nil}
	confirmCode := secureoperation.NewConfirmCode(
		&fakeTokenGen{token: "tok"},
		&fakeCodeGen{code: "code"},
		verifier,
	)

	op := newOpWithSingleTOTPAction(t, userID)

	out, commitConfirmed, err := confirmCode.Prepare(context.Background(), op, "123456")
	require.NoError(t, err)
	require.True(t, out.Is(operationstatus.Confirmed))
	require.Nil(t, commitConfirmed)
	require.Equal(t, confirmmethod.TOTP, verifier.calledMethod)
	require.Equal(t, userID, verifier.calledUserID)
}

func TestConfirmCode_TOTPVerifiedWithConsume(t *testing.T) {
	t.Parallel()

	called := false
	consume := func(_ context.Context) error {
		called = true

		return nil
	}

	verifier := &fakeVerifier{ok: true, consume: consume}
	confirmCode := secureoperation.NewConfirmCode(
		&fakeTokenGen{token: "tok"},
		&fakeCodeGen{code: "code"},
		verifier,
	)

	op := newOpWithSingleTOTPAction(t, uuid.New())

	out, commitConfirmed, err := confirmCode.Prepare(context.Background(), op, "recovery")
	require.NoError(t, err)
	require.True(t, out.Is(operationstatus.Confirmed))
	require.NotNil(t, commitConfirmed)

	require.NoError(t, commitConfirmed(context.Background()))
	require.True(t, called)
}

func TestConfirmCode_TOTPVerifierRejects(t *testing.T) {
	t.Parallel()

	verifier := &fakeVerifier{ok: false}
	confirmCode := secureoperation.NewConfirmCode(
		&fakeTokenGen{token: "tok"},
		&fakeCodeGen{code: "code"},
		verifier,
	)

	op := newOpWithSingleTOTPAction(t, uuid.New())

	out, commitConfirmed, err := confirmCode.Prepare(context.Background(), op, "bad")
	require.ErrorIs(t, err, secureoperation_model.ErrConfirmCodeIsIncorrect)
	require.False(t, out.Is(operationstatus.Confirmed))
	require.Nil(t, commitConfirmed)
}

func TestConfirmCode_TOTPVerifierError(t *testing.T) {
	t.Parallel()

	wantErr := secureoperation_model.ErrOperationAlreadyExpired
	verifier := &fakeVerifier{err: wantErr}
	confirmCode := secureoperation.NewConfirmCode(
		&fakeTokenGen{token: "tok"},
		&fakeCodeGen{code: "code"},
		verifier,
	)

	op := newOpWithSingleTOTPAction(t, uuid.New())

	out, commitConfirmed, err := confirmCode.Prepare(context.Background(), op, "any")
	require.ErrorIs(t, err, wantErr)
	require.False(t, out.Is(operationstatus.Confirmed))
	require.Nil(t, commitConfirmed)
}
