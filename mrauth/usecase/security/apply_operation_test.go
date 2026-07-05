package security_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
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

	uc := security.NewApplyOperation(fakeTx{}, &fakeOpVerifier{}, nil)

	require.Error(t, uc.Execute(context.Background(), uuid.Nil, "op-token"))
}

func TestApplyOperation_Success(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	storage := &fakeOpVerifier{op: confirmedOp(userID, "{}")}
	handler := &fakeHandler{}
	uc := security.NewApplyOperation(
		fakeTx{},
		storage,
		map[string]mrauth.OperationHandler{"confirm.change.totp": handler},
	)

	require.NoError(t, uc.Execute(context.Background(), userID, "op-token"))
	require.True(t, handler.executed)
	require.Equal(t, "op-token", storage.deletedToken)
}

func TestApplyOperation_WrongUser(t *testing.T) {
	t.Parallel()

	storage := &fakeOpVerifier{op: confirmedOp(uuid.New(), "{}")}
	uc := security.NewApplyOperation(fakeTx{}, storage, nil)

	require.Error(t, uc.Execute(context.Background(), uuid.New(), "op-token"))
}

func TestApplyOperation_NotConfirmed(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	op := secureoperation.SecureOperation{UserID: userID, Status: operationstatus.Opened}
	uc := security.NewApplyOperation(fakeTx{}, &fakeOpVerifier{op: op}, nil)

	require.Error(t, uc.Execute(context.Background(), userID, "op-token"))
}

func TestApplyOperation_UnknownName(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	storage := &fakeOpVerifier{op: confirmedOp(userID, "{}")}
	uc := security.NewApplyOperation(fakeTx{}, storage, map[string]mrauth.OperationHandler{})

	require.Error(t, uc.Execute(context.Background(), userID, "op-token"))
}
