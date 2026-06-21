package operation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
)

type (
	fakeTx struct{}

	fakeNotifier struct {
		sent int
		err  error
	}

	// fakeStorage - реализует все интерфейсы хранилища, используемые use-case'ами пакета.
	fakeStorage struct {
		fetchOp  secureoperation.SecureOperation
		fetchErr error

		replaced    bool
		replaceErr  error
		deleted     bool
		deleteErr   error
		insertCalls int
		insertErr   error

		updateCalled   bool
		updateAttempts int16
		updateErr      error
	}

	fakeConfirmPreparer struct {
		outOp  secureoperation.SecureOperation
		commit func(ctx context.Context) error
		err    error
	}

	fakeResendPreparer struct {
		outOp secureoperation.SecureOperation
		err   error
	}
)

func (fakeTx) Do(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
	return job(ctx)
}

func (f *fakeNotifier) Send(context.Context, string, map[string]any) error {
	if f.err != nil {
		return f.err
	}

	f.sent++

	return nil
}

func (f *fakeStorage) FetchOne(context.Context, string) (secureoperation.SecureOperation, error) {
	return f.fetchOp, f.fetchErr
}

func (f *fakeStorage) FetchOneForUpdate(ctx context.Context, token string) (secureoperation.SecureOperation, error) {
	return f.FetchOne(ctx, token)
}

func (f *fakeStorage) Replace(_ context.Context, _ string, _ secureoperation.SecureOperation) error {
	if f.replaceErr != nil {
		return f.replaceErr
	}

	f.replaced = true

	return nil
}

func (f *fakeStorage) UpdateFailedAttempt(context.Context, string) (int16, error) {
	f.updateCalled = true

	return f.updateAttempts, f.updateErr
}

func (f *fakeStorage) Delete(context.Context, string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}

	f.deleted = true

	return nil
}

func (f *fakeStorage) Insert(context.Context, []entity.SecureOperationLog) error {
	if f.insertErr != nil {
		return f.insertErr
	}

	f.insertCalls++

	return nil
}

func (p fakeConfirmPreparer) Prepare(
	context.Context,
	secureoperation.SecureOperation,
	string,
) (secureoperation.SecureOperation, func(ctx context.Context) error, error) {
	return p.outOp, p.commit, p.err
}

func (p fakeResendPreparer) Prepare(secureoperation.SecureOperation) (secureoperation.SecureOperation, error) {
	return p.outOp, p.err
}

func openedEmailOp(t *testing.T) secureoperation.SecureOperation {
	t.Helper()

	op, err := secureoperation.NewOperation(
		"token",
		"op.name",
		uuid.New(),
		[]secureoperation.ConfirmAction{
			{
				Method:           confirmmethod.Email,
				MaxAttempts:      3,
				MaxResends:       5,
				MinResendTime:    5 * time.Minute,
				Expiry:           10 * time.Minute,
				Address:          "u@e",
				ConfirmCode:      "code123",
				PlainConfirmCode: "code123",
			},
		},
		nil,
	)
	require.NoError(t, err)

	return op
}

func confirmedOp(t *testing.T) secureoperation.SecureOperation {
	t.Helper()

	op := secureoperation.SecureOperation{
		Token:     "token",
		Name:      "op.name",
		UserID:    uuid.New(),
		Status:    operationstatus.Confirmed,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	require.NoError(t, secureoperation.WakeUp(&op, nil))

	return op
}

func TestConfirmOperation_EmptyToken(t *testing.T) {
	t.Parallel()

	uc := operation.NewConfirmOperation(fakeTx{}, &fakeStorage{}, &fakeNotifier{}, fakeConfirmPreparer{})

	_, err := uc.Execute(context.Background(), "en", "", "code")
	require.Error(t, err)
}

func TestConfirmOperation_FetchError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fetch failed")
	uc := operation.NewConfirmOperation(fakeTx{}, &fakeStorage{fetchErr: wantErr}, &fakeNotifier{}, fakeConfirmPreparer{})

	_, err := uc.Execute(context.Background(), "en", "token", "code")
	require.ErrorIs(t, err, wantErr)
}

func TestConfirmOperation_NoAttempts(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	preparer := fakeConfirmPreparer{outOp: op, err: secureoperation.ErrNoAttemptsToConfirmOperation}
	uc := operation.NewConfirmOperation(fakeTx{}, &fakeStorage{fetchOp: op}, &fakeNotifier{}, preparer)

	_, err := uc.Execute(context.Background(), "en", "token", "code")
	require.ErrorIs(t, err, secureoperation.ErrNoAttemptsToConfirmOperation)
}

func TestConfirmOperation_WrongCode_AttemptsRemain(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	storage := &fakeStorage{fetchOp: op, updateAttempts: 2}
	preparer := fakeConfirmPreparer{outOp: op, err: secureoperation.ErrConfirmCodeIsIncorrect}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, &fakeNotifier{}, preparer)

	out, err := uc.Execute(context.Background(), "en", "token", "bad")
	require.ErrorIs(t, err, secureoperation.ErrConfirmCodeIsIncorrect)
	require.True(t, storage.updateCalled)
	require.Equal(t, int16(2), out.RemainingAttempts)
}

func TestConfirmOperation_WrongCode_NoAttemptsLeft(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	storage := &fakeStorage{fetchOp: op, updateAttempts: 0}
	preparer := fakeConfirmPreparer{outOp: op, err: secureoperation.ErrConfirmCodeIsIncorrect}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, &fakeNotifier{}, preparer)

	_, err := uc.Execute(context.Background(), "en", "token", "bad")
	require.ErrorIs(t, err, secureoperation.ErrNoAttemptsToConfirmOperation)
}

func TestConfirmOperation_Success_NotConfirmedNotifies(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	storage := &fakeStorage{fetchOp: op}
	notifier := &fakeNotifier{}
	preparer := fakeConfirmPreparer{outOp: op}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, notifier, preparer)

	_, err := uc.Execute(context.Background(), "en", "token", "code123")
	require.NoError(t, err)
	require.True(t, storage.replaced)
	require.Equal(t, 1, notifier.sent)
}

func TestConfirmOperation_Success_ConfirmedRunsCommit(t *testing.T) {
	t.Parallel()

	op := confirmedOp(t)
	committed := false
	storage := &fakeStorage{fetchOp: op}
	notifier := &fakeNotifier{}
	preparer := fakeConfirmPreparer{
		outOp: op,
		commit: func(context.Context) error {
			committed = true

			return nil
		},
	}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, notifier, preparer)

	_, err := uc.Execute(context.Background(), "en", "token", "code123")
	require.NoError(t, err)
	require.True(t, storage.replaced)
	require.True(t, committed)
	require.Equal(t, 0, notifier.sent) // подтверждённая операция не отправляет код
}

func TestConfirmOperation_Success_SecondFactorRaceRejectedAsWrongCode(t *testing.T) {
	t.Parallel()

	op := confirmedOp(t)
	storage := &fakeStorage{fetchOp: op}
	notifier := &fakeNotifier{}
	preparer := fakeConfirmPreparer{
		outOp: op,
		// второй фактор уже израсходован конкурентным подтверждением
		commit: func(context.Context) error {
			return sysmesserrors.ErrEventStorageNoRecordFound
		},
	}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, notifier, preparer)

	gotOp, err := uc.Execute(context.Background(), "en", "token", "code123")
	require.ErrorIs(t, err, secureoperation.ErrConfirmCodeIsIncorrect) // гонка отдаётся как неверный код
	require.NotErrorIs(t, err, sysmesserrors.ErrEventStorageNoRecordFound)
	require.Equal(t, secureoperation.SecureOperation{}, gotOp) // транзакция откатилась
	require.Equal(t, 0, notifier.sent)
}

func TestResendCode_EmptyToken(t *testing.T) {
	t.Parallel()

	uc := operation.NewResendCode(fakeTx{}, &fakeStorage{}, &fakeNotifier{}, fakeResendPreparer{})

	_, err := uc.Execute(context.Background(), "en", "")
	require.Error(t, err)
}

func TestResendCode_Restricted(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	preparer := fakeResendPreparer{outOp: op, err: secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted}
	uc := operation.NewResendCode(fakeTx{}, &fakeStorage{fetchOp: op}, &fakeNotifier{}, preparer)

	_, err := uc.Execute(context.Background(), "en", "token")
	require.ErrorIs(t, err, secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted)
}

func TestResendCode_Success(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	storage := &fakeStorage{fetchOp: op}
	notifier := &fakeNotifier{}
	preparer := fakeResendPreparer{outOp: op}
	uc := operation.NewResendCode(fakeTx{}, storage, notifier, preparer)

	_, err := uc.Execute(context.Background(), "en", "token")
	require.NoError(t, err)
	require.True(t, storage.replaced)
	require.Equal(t, 1, notifier.sent)
}

func TestRevokeOperation(t *testing.T) {
	t.Parallel()

	t.Run("empty token", func(t *testing.T) {
		t.Parallel()

		err := operation.NewRevokeOperation(&fakeStorage{}).Execute(context.Background(), "")
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		storage := &fakeStorage{}
		require.NoError(t, operation.NewRevokeOperation(storage).Execute(context.Background(), "token"))
		assert.True(t, storage.deleted)
	})

	t.Run("delete error", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("delete failed")
		err := operation.NewRevokeOperation(&fakeStorage{deleteErr: wantErr}).Execute(context.Background(), "token")
		require.ErrorIs(t, err, wantErr)
	})
}

func TestStatistic(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		storage := &fakeStorage{}
		require.NoError(t, operation.NewStatistic(storage).Execute(context.Background(), []entity.SecureOperationLog{}))
		assert.Equal(t, 1, storage.insertCalls)
	})

	t.Run("insert error", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("insert failed")
		err := operation.NewStatistic(&fakeStorage{insertErr: wantErr}).Execute(context.Background(), nil)
		require.ErrorIs(t, err, wantErr)
	})
}
