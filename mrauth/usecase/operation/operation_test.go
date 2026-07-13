package operation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
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

	fakeOperationLogger struct {
		entries []entity.SecureOperationLog
	}
)

func (f *fakeOperationLogger) Log(_ context.Context, entry entity.SecureOperationLog) {
	f.entries = append(f.entries, entry)
}

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

	uc := operation.NewConfirmOperation(fakeTx{}, &fakeStorage{}, &fakeNotifier{}, fakeConfirmPreparer{}, &fakeOperationLogger{})

	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "", "code")
	require.Error(t, err)
}

func TestConfirmOperation_FetchError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fetch failed")
	uc := operation.NewConfirmOperation(fakeTx{}, &fakeStorage{fetchErr: wantErr}, &fakeNotifier{}, fakeConfirmPreparer{}, &fakeOperationLogger{})

	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "token", "code")
	require.ErrorIs(t, err, wantErr)
}

func TestConfirmOperation_NoAttempts(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	preparer := fakeConfirmPreparer{outOp: op, err: secureoperation.ErrNoAttemptsToConfirmOperation}
	logOperation := &fakeOperationLogger{}
	uc := operation.NewConfirmOperation(fakeTx{}, &fakeStorage{fetchOp: op}, &fakeNotifier{}, preparer, logOperation)

	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "token", "code")
	require.ErrorIs(t, err, secureoperation.ErrNoAttemptsToConfirmOperation)
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.Blocked, logOperation.entries[0].LogStatus)
	require.Equal(t, logreason.AttemptsExhausted, logOperation.entries[0].Reason)
}

func TestConfirmOperation_WrongCode_AttemptsRemain(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	storage := &fakeStorage{fetchOp: op, updateAttempts: 2}
	preparer := fakeConfirmPreparer{outOp: op, err: secureoperation.ErrConfirmCodeIsIncorrect}
	logOperation := &fakeOperationLogger{}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, &fakeNotifier{}, preparer, logOperation)

	out, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "token", "bad")
	require.ErrorIs(t, err, secureoperation.ErrConfirmCodeIsIncorrect)
	require.True(t, storage.updateCalled)
	require.Equal(t, int16(2), out.RemainingAttempts)
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.ConfirmFailed, logOperation.entries[0].LogStatus)
	require.Equal(t, logreason.WrongCode, logOperation.entries[0].Reason)
	// поток анонимный, но владелец операции известен после её чтения
	require.Equal(t, op.UserID, logOperation.entries[0].VisitorID)
}

func TestConfirmOperation_WrongCode_NoAttemptsLeft(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	storage := &fakeStorage{fetchOp: op, updateAttempts: 0}
	preparer := fakeConfirmPreparer{outOp: op, err: secureoperation.ErrConfirmCodeIsIncorrect}
	logOperation := &fakeOperationLogger{}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, &fakeNotifier{}, preparer, logOperation)

	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "token", "bad")
	require.ErrorIs(t, err, secureoperation.ErrNoAttemptsToConfirmOperation)
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.Blocked, logOperation.entries[0].LogStatus)
	require.Equal(t, logreason.AttemptsExhausted, logOperation.entries[0].Reason)
}

func TestConfirmOperation_Success_NotConfirmedNotifies(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	storage := &fakeStorage{fetchOp: op}
	notifier := &fakeNotifier{}
	preparer := fakeConfirmPreparer{outOp: op}
	logOperation := &fakeOperationLogger{}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, notifier, preparer, logOperation)

	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "token", "code123")
	require.NoError(t, err)
	require.True(t, storage.replaced)
	require.Equal(t, 1, notifier.sent)
	// действие подтверждено, но операция ещё не завершена: только CONFIRM_SUCCESS
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.ConfirmSuccess, logOperation.entries[0].LogStatus)
}

func TestConfirmOperation_Success_ConfirmedRunsCommit(t *testing.T) {
	t.Parallel()

	committed := false
	storage := &fakeStorage{fetchOp: openedEmailOp(t)} // в хранилище операция ещё Opened
	notifier := &fakeNotifier{}
	preparer := fakeConfirmPreparer{
		outOp: confirmedOp(t), // Prepare подтверждает её и возвращает commit второго фактора
		commit: func(context.Context) error {
			committed = true

			return nil
		},
	}
	logOperation := &fakeOperationLogger{}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, notifier, preparer, logOperation)

	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "token", "code123")
	require.NoError(t, err)
	require.True(t, storage.replaced)
	require.True(t, committed)
	require.Equal(t, 0, notifier.sent) // подтверждённая операция не отправляет код
	// финальное подтверждение фиксируется одной записью CONFIRMED (без отдельной CONFIRM_SUCCESS)
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.Confirmed, logOperation.entries[0].LogStatus)
	require.Equal(t, logreason.Unspecified, logOperation.entries[0].Reason)
}

// TestConfirmOperation_AlreadyConfirmedIsIdempotent - повторное подтверждение уже подтверждённой
// операции замыкается накоротко: Prepare/Replace не вызываются, операция возвращается как успех
// (нужно, чтобы поток открытия сессии можно было безопасно повторить после сбоя сессии).
func TestConfirmOperation_AlreadyConfirmedIsIdempotent(t *testing.T) {
	t.Parallel()

	storage := &fakeStorage{fetchOp: confirmedOp(t)}
	notifier := &fakeNotifier{}
	// preparer вернул бы ошибку, если бы был вызван - проверяем, что короткое замыкание сработало
	preparer := fakeConfirmPreparer{err: errors.New("Prepare must not be called")}
	logOperation := &fakeOperationLogger{}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, notifier, preparer, logOperation)

	out, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "token", "code123")
	require.NoError(t, err)
	assert.True(t, out.Is(operationstatus.Confirmed))
	assert.False(t, storage.replaced)
	assert.Equal(t, 0, notifier.sent)
	// повтор ничего не меняет, поэтому и в журнал не пишется: событие CONFIRMED уже
	// зафиксировано при первом подтверждении
	assert.Empty(t, logOperation.entries)
}

func TestConfirmOperation_Success_Auth2FARaceRejectedAsWrongCode(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t) // в хранилище операция ещё Opened
	storage := &fakeStorage{fetchOp: op}
	notifier := &fakeNotifier{}
	preparer := fakeConfirmPreparer{
		outOp: confirmedOp(t),
		// второй фактор уже израсходован конкурентным подтверждением
		commit: func(context.Context) error {
			return sysmesserrors.ErrEventStorageNoRecordFound
		},
	}
	logOperation := &fakeOperationLogger{}
	uc := operation.NewConfirmOperation(fakeTx{}, storage, notifier, preparer, logOperation)

	gotOp, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "token", "code123")
	require.ErrorIs(t, err, secureoperation.ErrConfirmCodeIsIncorrect) // гонка отдаётся как неверный код
	require.NotErrorIs(t, err, sysmesserrors.ErrEventStorageNoRecordFound)
	require.Equal(t, secureoperation.SecureOperation{}, gotOp) // транзакция откатилась
	require.Equal(t, 0, notifier.sent)
	// TOTP-replay фиксируется в журнале даже при откате транзакции
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.ConfirmFailed, logOperation.entries[0].LogStatus)
	require.Equal(t, logreason.TOTPReplay, logOperation.entries[0].Reason)
	require.Equal(t, op.UserID, logOperation.entries[0].VisitorID)
}

func TestResendCode_EmptyToken(t *testing.T) {
	t.Parallel()

	uc := operation.NewResendCode(fakeTx{}, &fakeStorage{}, &fakeNotifier{}, fakeResendPreparer{}, &fakeOperationLogger{})

	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "")
	require.Error(t, err)
}

func TestResendCode_Restricted(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	preparer := fakeResendPreparer{outOp: op, err: secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted}
	logOperation := &fakeOperationLogger{}
	uc := operation.NewResendCode(fakeTx{}, &fakeStorage{fetchOp: op}, &fakeNotifier{}, preparer, logOperation)

	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "token")
	require.ErrorIs(t, err, secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted)
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.Blocked, logOperation.entries[0].LogStatus)
	require.Equal(t, logreason.Throttled, logOperation.entries[0].Reason)
}

func TestResendCode_Success(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	storage := &fakeStorage{fetchOp: op}
	notifier := &fakeNotifier{}
	preparer := fakeResendPreparer{outOp: op}
	logOperation := &fakeOperationLogger{}
	uc := operation.NewResendCode(fakeTx{}, storage, notifier, preparer, logOperation)

	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "en", "token")
	require.NoError(t, err)
	require.True(t, storage.replaced)
	require.Equal(t, 1, notifier.sent)
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.ResentCode, logOperation.entries[0].LogStatus)
	// поток анонимный, но владелец операции известен после её чтения
	require.Equal(t, op.UserID, logOperation.entries[0].VisitorID)
}

func TestRevokeOperation_EmptyToken(t *testing.T) {
	t.Parallel()

	err := operation.NewRevokeOperation(&fakeStorage{}, &fakeOperationLogger{}).Execute(context.Background(), dto.ActorMeta{}, "")
	require.Error(t, err)
}

func TestRevokeOperation_Success(t *testing.T) {
	t.Parallel()

	op := openedEmailOp(t)
	storage := &fakeStorage{fetchOp: op}
	logOperation := &fakeOperationLogger{}
	require.NoError(t, operation.NewRevokeOperation(storage, logOperation).Execute(context.Background(), dto.ActorMeta{}, "token"))
	assert.True(t, storage.deleted)
	// операция читается перед удалением, поэтому в журнал попадает, что именно отозвано
	require.Len(t, logOperation.entries, 1)
	assert.Equal(t, logstatus.Revoked, logOperation.entries[0].LogStatus)
	assert.Equal(t, op.Name, logOperation.entries[0].OperationName)
	assert.Equal(t, confirmmethod.Email, logOperation.entries[0].ConfirmMethod)
	assert.Equal(t, op.UserID, logOperation.entries[0].VisitorID)
}

func TestRevokeOperation_FetchError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fetch failed")
	storage := &fakeStorage{fetchErr: wantErr}
	logOperation := &fakeOperationLogger{}

	err := operation.NewRevokeOperation(storage, logOperation).Execute(context.Background(), dto.ActorMeta{}, "token")
	require.ErrorIs(t, err, wantErr)
	assert.False(t, storage.deleted)
	assert.Empty(t, logOperation.entries)
}

func TestRevokeOperation_DeleteError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("delete failed")
	logOperation := &fakeOperationLogger{}

	err := operation.NewRevokeOperation(&fakeStorage{deleteErr: wantErr}, logOperation).Execute(context.Background(), dto.ActorMeta{}, "token")
	require.ErrorIs(t, err, wantErr)
	assert.Empty(t, logOperation.entries)
}

func TestStatistic_Success(t *testing.T) {
	t.Parallel()

	storage := &fakeStorage{}
	require.NoError(t, operation.NewStatistic(storage).Execute(context.Background(), []entity.SecureOperationLog{}))
	assert.Equal(t, 1, storage.insertCalls)
}

func TestStatistic_InsertError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("insert failed")
	err := operation.NewStatistic(&fakeStorage{insertErr: wantErr}).Execute(context.Background(), nil)
	require.ErrorIs(t, err, wantErr)
}
