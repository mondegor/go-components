package security_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/security"
)

// testTotpSecret - валидный base32 TOTP-secret, используемый в тестах verify_totp.
const testTotpSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

type (
	// fakeTx - реализует mrstorage.DBTxManager простым вызовом переданной job.
	fakeTx struct{}

	// fakeBinder - фиксирует переданную для сохранения запись Auth2FA.
	fakeBinder struct {
		saved entity.Auth2FA
		err   error
	}

	// fakeOpVerifier - возвращает заранее заданную операцию и фиксирует токен удаления.
	fakeOpVerifier struct {
		op           secureoperation.SecureOperation
		fetchErr     error
		deleteErr    error
		deletedToken string
	}

	// fakeNotifier - фиксирует факт отправки уведомления.
	fakeNotifier struct {
		sent bool
		err  error
	}
)

func (fakeTx) Do(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
	return job(ctx)
}

func (f *fakeBinder) InsertOrUpdate(_ context.Context, row entity.Auth2FA) error {
	if f.err != nil {
		return f.err
	}

	f.saved = row

	return nil
}

func (f *fakeOpVerifier) FetchOne(_ context.Context, _ string) (secureoperation.SecureOperation, error) {
	return f.op, f.fetchErr
}

func (f *fakeOpVerifier) FetchOneForUpdate(_ context.Context, _ string) (secureoperation.SecureOperation, error) {
	return f.op, f.fetchErr
}

func (f *fakeOpVerifier) Delete(_ context.Context, token string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}

	f.deletedToken = token

	return nil
}

func (f *fakeNotifier) Send(_ context.Context, _ string, _ map[string]any) error {
	if f.err != nil {
		return f.err
	}

	f.sent = true

	return nil
}

func TestVerifyTOTPGenerator_ValidCode_BindsAndReturnsCodes(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	secret := testTotpSecret
	op := confirmedOp(userID, `{"email":"u@e","secret":"`+secret+`"}`)

	binder := &fakeBinder{}
	verifier := &fakeOpVerifier{op: op}
	notifier := &fakeNotifier{}
	gen := crypt.NewSecretGenerator(10)
	auth := totp.NewAuthenticator("TestIssuer", 20)

	uc := security.NewApplyTOTPGenerator(fakeTx{}, binder, verifier, gen, auth, notifier, 10)

	code, err := auth.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	codes, err := uc.Execute(context.Background(), userID, "op-token", code)
	require.NoError(t, err)
	require.Len(t, codes, 10)
	require.Equal(t, auth2fatype.TOTP, binder.saved.Type)
	require.Equal(t, secret, binder.saved.Secret)
	require.Len(t, binder.saved.RecoveryCodes, 10)
	require.NotEqual(t, codes, binder.saved.RecoveryCodes) // хранятся хеши, возвращается plaintext
	require.Equal(t, "op-token", verifier.deletedToken)
	require.True(t, notifier.sent)
}

func TestVerifyTOTPGenerator_InvalidCode_NoBind(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	op := confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`)

	binder := &fakeBinder{}
	verifier := &fakeOpVerifier{op: op}
	notifier := &fakeNotifier{}
	gen := crypt.NewSecretGenerator(10)
	auth := totp.NewAuthenticator("TestIssuer", 20)

	uc := security.NewApplyTOTPGenerator(fakeTx{}, binder, verifier, gen, auth, notifier, 10)

	codes, err := uc.Execute(context.Background(), userID, "op-token", "000000")
	require.Error(t, err)
	require.Nil(t, codes)
	require.Equal(t, entity.Auth2FA{}, binder.saved)
	require.Empty(t, verifier.deletedToken)
	require.False(t, notifier.sent)
}
