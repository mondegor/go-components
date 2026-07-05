package security_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/usecase/security"
)

func confirmedPasswordOp(userID uuid.UUID, payload string) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Token:   "op-token",
		Name:    unit.NameConfirmChangePassword,
		UserID:  userID,
		Payload: []byte(payload),
		Status:  operationstatus.Confirmed,
	}
}

func TestApplyPassword_Confirmed_BindsAndReturnsCodes(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	op := confirmedPasswordOp(userID, `{"new_password":"hashed-pwd","email":"u@e"}`)

	binder := &fakeBinder{}
	verifier := &fakeOpVerifier{op: op}
	notifier := &fakeNotifier{}

	uc := security.NewApplyPassword(fakeTx{}, binder, verifier, crypt.NewSecretGenerator(10), notifier, 8)

	codes, err := uc.Execute(context.Background(), userID, "op-token")
	require.NoError(t, err)
	require.Len(t, codes, 8)
	require.Equal(t, auth2fatype.Password, binder.saved.Type)
	require.Equal(t, "hashed-pwd", binder.saved.Secret) // секрет уже захеширован при создании операции
	require.Len(t, binder.saved.RecoveryCodes, 8)
	require.NotEqual(t, codes, binder.saved.RecoveryCodes) // хранятся хеши, возвращается plaintext
	require.Equal(t, "op-token", verifier.deletedToken)
	require.True(t, notifier.sent)
}

func TestApplyPassword_ReissuesNewCodesEachTime(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	payload := `{"new_password":"hashed-pwd","email":"u@e"}`
	gen := crypt.NewSecretGenerator(10)

	uc := security.NewApplyPassword(fakeTx{}, &fakeBinder{}, &fakeOpVerifier{op: confirmedPasswordOp(userID, payload)}, gen, &fakeNotifier{}, 8)

	first, err := uc.Execute(context.Background(), userID, "op-token")
	require.NoError(t, err)

	second, err := uc.Execute(context.Background(), userID, "op-token")
	require.NoError(t, err)

	require.NotEqual(t, first, second) // каждая смена пароля выдаёт новый набор кодов
}

func TestApplyPassword_WrongOperationName_NoBind(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	// операция чужого типа (confirm.change.totp) не должна применяться как смена пароля
	op := confirmedOp(userID, `{"new_password":"hashed-pwd","email":"u@e"}`)

	binder := &fakeBinder{}
	verifier := &fakeOpVerifier{op: op}
	notifier := &fakeNotifier{}

	uc := security.NewApplyPassword(fakeTx{}, binder, verifier, crypt.NewSecretGenerator(10), notifier, 8)

	codes, err := uc.Execute(context.Background(), userID, "op-token")
	require.Error(t, err)
	require.Nil(t, codes)
	require.Empty(t, verifier.deletedToken)
	require.False(t, notifier.sent)
}
