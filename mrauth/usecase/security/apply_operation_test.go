package security_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/usecase/security"
)

type fakeHandler struct {
	executed bool
	err      error
}

func (f *fakeHandler) Execute(context.Context, uuid.UUID, []byte) error {
	if f.err != nil {
		return f.err
	}

	f.executed = true

	return nil
}

func TestApplyOperation_NilUserID(t *testing.T) {
	t.Parallel()

	uc := security.NewApplyOperation(fakeTx{}, &fakeOpVerifier{}, &fakeOperationLogger{}, nil)

	require.Error(t, uc.Execute(context.Background(), dto.ActorMeta{}, "op-token"))
}

func TestApplyOperation_Success(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	op := confirmedOp(userID, "{}")
	storage := &fakeOpVerifier{op: op}
	handler := &fakeHandler{}
	logOperation := &fakeOperationLogger{}
	uc := security.NewApplyOperation(
		fakeTx{},
		storage,
		logOperation,
		map[string]mrauth.OperationHandler{"confirm.change.totp": handler},
	)

	require.NoError(t, uc.Execute(context.Background(), dto.ActorMeta{VisitorID: userID}, "op-token"))
	require.True(t, handler.executed)
	require.Equal(t, "op-token", storage.deletedToken)
	require.Len(t, logOperation.entries, 1)
	assert.Equal(t, logstatus.Applied, logOperation.entries[0].LogStatus)
	assert.Equal(t, logreason.Unspecified, logOperation.entries[0].Reason)
	assert.Equal(t, op.Name, logOperation.entries[0].OperationName)
	assert.Equal(t, userID, logOperation.entries[0].VisitorID)
}

func TestApplyOperation_WrongUser(t *testing.T) {
	t.Parallel()

	stranger := uuid.New()
	storage := &fakeOpVerifier{op: confirmedOp(uuid.New(), "{}")}
	logOperation := &fakeOperationLogger{}
	uc := security.NewApplyOperation(fakeTx{}, storage, logOperation, nil)

	require.Error(t, uc.Execute(context.Background(), dto.ActorMeta{VisitorID: stranger}, "op-token"))

	// в журнал попадает обратившийся, а не владелец операции
	require.Len(t, logOperation.entries, 1)
	assert.Equal(t, logstatus.Blocked, logOperation.entries[0].LogStatus)
	assert.Equal(t, logreason.AccessForbidden, logOperation.entries[0].Reason)
	assert.Equal(t, stranger, logOperation.entries[0].VisitorID)
}

func TestApplyOperation_NotConfirmed(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	op := confirmedOp(userID, "{}")
	op.Status = operationstatus.Opened
	logOperation := &fakeOperationLogger{}
	uc := security.NewApplyOperation(
		fakeTx{},
		&fakeOpVerifier{op: op},
		logOperation,
		map[string]mrauth.OperationHandler{"confirm.change.totp": &fakeHandler{}},
	)

	require.Error(t, uc.Execute(context.Background(), dto.ActorMeta{VisitorID: userID}, "op-token"))
	require.Len(t, logOperation.entries, 1)
	assert.Equal(t, logstatus.Blocked, logOperation.entries[0].LogStatus)
	assert.Equal(t, logreason.NotConfirmed, logOperation.entries[0].Reason)
}

func TestApplyOperation_UnknownName(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	storage := &fakeOpVerifier{op: confirmedOp(userID, "{}")}
	logOperation := &fakeOperationLogger{}
	uc := security.NewApplyOperation(fakeTx{}, storage, logOperation, map[string]mrauth.OperationHandler{})

	require.Error(t, uc.Execute(context.Background(), dto.ActorMeta{VisitorID: userID}, "op-token"))

	// незарегистрированный обработчик - ошибка конфигурации, а не событие безопасности
	assert.Empty(t, logOperation.entries)
}
