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

func emailConfirmAction(code string) secureoperation_model.ConfirmAction {
	return secureoperation_model.ConfirmAction{
		Method:        confirmmethod.Email,
		MaxAttempts:   3,
		MaxResends:    5,
		MinResendTime: 5 * time.Minute,
		Expiry:        10 * time.Minute,
		Address:       "u@e",
		ConfirmCode:   code,
	}
}

func TestConfirmCode_EmailCorrectCodeConfirms(t *testing.T) {
	t.Parallel()

	op, err := secureoperation_model.NewOperation(
		"token",
		"name1",
		uuid.New(),
		[]secureoperation_model.ConfirmAction{emailConfirmAction("secret1")},
		nil,
	)
	require.NoError(t, err)

	confirmCode := secureoperation.NewConfirmCode(&fakeTokenGen{token: "tok"}, &fakeCodeGen{code: "code"}, &fakeVerifier{})

	out, commit, err := confirmCode.Prepare(context.Background(), op, "secret1")
	require.NoError(t, err)
	require.True(t, out.Is(operationstatus.Confirmed))
	require.Nil(t, commit)
}

func TestConfirmCode_EmailWrongCodeRejected(t *testing.T) {
	t.Parallel()

	op, err := secureoperation_model.NewOperation(
		"token",
		"name1",
		uuid.New(),
		[]secureoperation_model.ConfirmAction{emailConfirmAction("secret1")},
		nil,
	)
	require.NoError(t, err)

	confirmCode := secureoperation.NewConfirmCode(&fakeTokenGen{token: "tok"}, &fakeCodeGen{code: "code"}, &fakeVerifier{})

	out, commit, err := confirmCode.Prepare(context.Background(), op, "wrong")
	require.ErrorIs(t, err, secureoperation_model.ErrConfirmCodeIsIncorrect)
	require.False(t, out.Is(operationstatus.Confirmed))
	require.Nil(t, commit)
}

func TestConfirmCode_FirstOfTwoActionsGeneratesNextCode(t *testing.T) {
	t.Parallel()

	op, err := secureoperation_model.NewOperation(
		"token",
		"name1",
		uuid.New(),
		[]secureoperation_model.ConfirmAction{emailConfirmAction("secret1"), emailConfirmAction("secret2")},
		nil,
	)
	require.NoError(t, err)

	confirmCode := secureoperation.NewConfirmCode(&fakeTokenGen{token: "new-token"}, &fakeCodeGen{code: "new-code"}, &fakeVerifier{})

	out, _, err := confirmCode.Prepare(context.Background(), op, "secret1")
	require.NoError(t, err)
	require.False(t, out.Is(operationstatus.Confirmed))
	require.Equal(t, "new-token", out.Token)

	action, ok := out.FirstAction()
	require.True(t, ok)
	require.Equal(t, "new-code", action.ConfirmCode)
}
