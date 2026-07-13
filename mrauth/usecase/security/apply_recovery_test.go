package security_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/usecase/security"
)

// fakeRecoveryUpdater - фиксирует переданный новый набор аварийных кодов.
type fakeRecoveryUpdater struct {
	saved []string
	err   error
}

func (f *fakeRecoveryUpdater) UpdateRecoveryCodes(_ context.Context, _ uuid.UUID, hashed []string) error {
	if f.err != nil {
		return f.err
	}

	f.saved = hashed

	return nil
}

func confirmedRegenerateOp(userID uuid.UUID) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Token:   "op-token",
		Name:    unit.NameConfirmRegenerateRecovery,
		UserID:  userID,
		Payload: []byte(`{"email":"u@e"}`),
		Status:  operationstatus.Confirmed,
	}
}

func TestApplyRecovery_Confirmed_ReplacesAndReturnsCodes(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	updater := &fakeRecoveryUpdater{}
	verifier := &fakeOpVerifier{op: confirmedRegenerateOp(userID)}
	notifier := &fakeNotifier{}
	logOperation := &fakeOperationLogger{}

	uc := security.NewApplyRecovery(fakeTx{}, updater, verifier, crypt.NewSecretGenerator(10), notifier, logOperation, 8)

	codes, err := uc.Execute(context.Background(), dto.ActorMeta{VisitorID: userID}, "op-token")
	require.NoError(t, err)
	require.Len(t, codes, 8)
	require.Len(t, updater.saved, 8)
	require.NotEqual(t, codes, updater.saved) // хранятся хеши, возвращается plaintext
	require.Equal(t, "op-token", verifier.deletedToken)
	require.True(t, notifier.sent)
	require.Len(t, logOperation.entries, 1)
	assert.Equal(t, logstatus.Applied, logOperation.entries[0].LogStatus)
	assert.Equal(t, unit.NameConfirmRegenerateRecovery, logOperation.entries[0].OperationName)
}

func TestApplyRecovery_WrongOperationName_NoUpdate(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	// операция чужого типа (confirm.change.totp) не должна применяться как перевыпуск
	op := confirmedOp(userID, `{"email":"u@e"}`)

	updater := &fakeRecoveryUpdater{}
	verifier := &fakeOpVerifier{op: op}
	notifier := &fakeNotifier{}
	logOperation := &fakeOperationLogger{}

	uc := security.NewApplyRecovery(fakeTx{}, updater, verifier, crypt.NewSecretGenerator(10), notifier, logOperation, 8)

	codes, err := uc.Execute(context.Background(), dto.ActorMeta{VisitorID: userID}, "op-token")
	require.Error(t, err)
	require.Nil(t, codes)
	require.Nil(t, updater.saved)
	require.Empty(t, verifier.deletedToken)
	require.False(t, notifier.sent)
	require.Len(t, logOperation.entries, 1)
	assert.Equal(t, logstatus.Blocked, logOperation.entries[0].LogStatus)
	assert.Equal(t, logreason.AccessForbidden, logOperation.entries[0].Reason)
}
