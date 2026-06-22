package auth2fa_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/service/auth2fa"
)

const testTOTPSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

type fakeSource struct {
	row           entity.Auth2FA
	fetchErr      error
	consumeErr    error
	consumedHash  string
	consumeCalled bool
	remaining     int
	updateStepErr error
	updatedStep   int64
	updateCalled  bool
}

func (s *fakeSource) FetchOne(_ context.Context, _ uuid.UUID) (entity.Auth2FA, error) {
	if s.fetchErr != nil {
		return entity.Auth2FA{}, s.fetchErr
	}

	return s.row, nil
}

func (s *fakeSource) UpdateRecoveryCode(_ context.Context, _ uuid.UUID, hash string) (int, error) {
	s.consumeCalled = true
	s.consumedHash = hash

	return s.remaining, s.consumeErr
}

// fakeAlerter - фиксирует факт и аргумент вызова SendAlert.
type fakeAlerter struct {
	called    bool
	remaining int
	err       error
}

func (a *fakeAlerter) SendAlert(_ context.Context, _ uuid.UUID, codeRemaining int) error {
	a.called = true
	a.remaining = codeRemaining

	return a.err
}

func (s *fakeSource) UpdateTOTPStep(_ context.Context, _ uuid.UUID, step int64) error {
	s.updateCalled = true
	s.updatedStep = step

	return s.updateStepErr
}

// countingComparer - счётчик вызовов сравнения для проверки, что bcrypt не перебирается зря.
type countingComparer struct {
	calls int
}

func (c *countingComparer) CompareSecretAndHash(_, _ string) (bool, error) {
	c.calls++

	return false, errors.New("no match")
}

func TestVerifier_ValidTOTP(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	src := &fakeSource{row: entity.Auth2FA{UserID: userID, Type: auth2fatype.TOTP, Secret: testTOTPSecret}}

	auth := totp.NewAuthenticator("TestIssuer", 20)

	code, err := auth.GenerateCode(testTOTPSecret, time.Now())
	require.NoError(t, err)

	v := auth2fa.NewVerifier(src, crypt.NewSecretGenerator(10), auth)

	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.TOTP, code)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, commit) // успешный TOTP возвращает commit для фиксации использованного шага
	require.False(t, src.consumeCalled)

	require.NoError(t, commit(context.Background()))
	require.True(t, src.updateCalled)
	require.NotZero(t, src.updatedStep)
}

func TestVerifier_TOTPReplayRejected(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	auth := totp.NewAuthenticator("TestIssuer", 20)

	now := time.Now()

	code, err := auth.GenerateCode(testTOTPSecret, now)
	require.NoError(t, err)

	// последний использованный шаг заведомо не меньше текущего: код того же окна
	// должен быть отклонён как повтор (replay).
	src := &fakeSource{row: entity.Auth2FA{
		UserID:       userID,
		Type:         auth2fatype.TOTP,
		Secret:       testTOTPSecret,
		LastTOTPStep: now.Unix()/30 + 5,
	}}

	v := auth2fa.NewVerifier(src, crypt.NewSecretGenerator(10), auth)

	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.TOTP, code)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, commit)
	require.False(t, src.updateCalled)
}

func TestVerifier_RecoveryFallbackConsumes(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	gen := crypt.NewSecretGenerator(10)

	h1, err := gen.HashedSecret("AAAAABBBBB")
	require.NoError(t, err)

	h2, err := gen.HashedSecret("CCCCCDDDDD")
	require.NoError(t, err)

	src := &fakeSource{row: entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1, h2},
	}}

	v := auth2fa.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.TOTP, "AAAAABBBBB")
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, commit)
	require.False(t, src.consumeCalled)

	require.NoError(t, commit(context.Background()))
	require.True(t, src.consumeCalled)
	require.Equal(t, h1, src.consumedHash) // израсходован именно совпавший хеш
}

func TestVerifier_InvalidTOTPNoRecoveryMatch(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	gen := crypt.NewSecretGenerator(10)

	h1, err := gen.HashedSecret("AAAAABBBBB")
	require.NoError(t, err)

	src := &fakeSource{row: entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	}}

	v := auth2fa.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.TOTP, "ZZZZZYYYYY")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, commit)
	require.False(t, src.consumeCalled)
}

func TestVerifier_AllDigitCodeSkipsRecovery(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	comparer := &countingComparer{}

	src := &fakeSource{row: entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{"hash-1", "hash-2", "hash-3"},
	}}

	v := auth2fa.NewVerifier(src, comparer, totp.NewAuthenticator("TestIssuer", 20))

	// неверный код в формате TOTP (только цифры) не должен запускать перебор bcrypt-хешей
	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.TOTP, "000000")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, commit)
	require.Zero(t, comparer.calls)
}

func TestVerifier_RecoveryConsumeRace(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	gen := crypt.NewSecretGenerator(10)

	h1, err := gen.HashedSecret("AAAAABBBBB")
	require.NoError(t, err)

	// код уже израсходован параллельной операцией: UpdateRecoveryCode возвращает ошибку
	src := &fakeSource{
		row: entity.Auth2FA{
			UserID:        userID,
			Type:          auth2fatype.TOTP,
			Secret:        testTOTPSecret,
			RecoveryCodes: []string{h1},
		},
		consumeErr: errors.New("record not found"),
	}

	v := auth2fa.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.TOTP, "AAAAABBBBB")
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, commit)

	require.ErrorIs(t, commit(context.Background()), src.consumeErr)
}

func TestVerifier_PasswordCorrect(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	gen := crypt.NewSecretGenerator(10)

	hash, err := gen.HashedSecret("my-secret-password")
	require.NoError(t, err)

	src := &fakeSource{row: entity.Auth2FA{UserID: userID, Type: auth2fatype.Password, Secret: hash}}

	v := auth2fa.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.Password, "my-secret-password")
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, commit)
}

func TestVerifier_PasswordWrong(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	gen := crypt.NewSecretGenerator(10)

	hash, err := gen.HashedSecret("my-secret-password")
	require.NoError(t, err)

	src := &fakeSource{row: entity.Auth2FA{UserID: userID, Type: auth2fatype.Password, Secret: hash}}

	v := auth2fa.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.Password, "wrong-password")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, commit)
}

func TestVerifier_PasswordRecoveryFallbackConsumes(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	gen := crypt.NewSecretGenerator(10)

	pwdHash, err := gen.HashedSecret("my-secret-password")
	require.NoError(t, err)

	recHash, err := gen.HashedSecret("AAAAABBBBB")
	require.NoError(t, err)

	src := &fakeSource{row: entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.Password,
		Secret:        pwdHash,
		RecoveryCodes: []string{recHash},
	}}

	v := auth2fa.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

	// пароль не подошёл, но предъявлен валидный аварийный код - он засчитывается и расходуется
	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.Password, "AAAAABBBBB")
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, commit)
	require.False(t, src.consumeCalled)

	require.NoError(t, commit(context.Background()))
	require.True(t, src.consumeCalled)
	require.Equal(t, recHash, src.consumedHash)
}

func TestVerifier_PasswordWrongNoRecoveryMatch(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	gen := crypt.NewSecretGenerator(10)

	pwdHash, err := gen.HashedSecret("my-secret-password")
	require.NoError(t, err)

	recHash, err := gen.HashedSecret("AAAAABBBBB")
	require.NoError(t, err)

	src := &fakeSource{row: entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.Password,
		Secret:        pwdHash,
		RecoveryCodes: []string{recHash},
	}}

	v := auth2fa.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

	// ни пароль, ни аварийный код не совпали - доступ не предоставляется, код не расходуется
	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.Password, "ZZZZZYYYYY")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, commit)
	require.False(t, src.consumeCalled)
}

func TestVerifier_RecoveryConsumed_CallsAlerter(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	gen := crypt.NewSecretGenerator(10)

	h1, err := gen.HashedSecret("AAAAABBBBB")
	require.NoError(t, err)

	src := &fakeSource{
		row: entity.Auth2FA{
			UserID:        userID,
			Type:          auth2fatype.TOTP,
			Secret:        testTOTPSecret,
			RecoveryCodes: []string{h1},
		},
		remaining: 1, // после расхода остаётся 1 код
	}
	alerter := &fakeAlerter{}

	// Verifier всегда сообщает остаток alerter'у; решение о пороге - на стороне alerter
	v := auth2fa.NewVerifier(
		src, gen, totp.NewAuthenticator("TestIssuer", 20),
		auth2fa.WithRecoveryAlerter(alerter),
	)

	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.TOTP, "AAAAABBBBB")
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, commit)
	require.False(t, alerter.called) // вызов только после фиксации (commit)

	require.NoError(t, commit(context.Background()))
	require.True(t, alerter.called)
	require.Equal(t, 1, alerter.remaining)
}

func TestVerifier_FetchError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fetch failed")
	src := &fakeSource{fetchErr: wantErr}

	v := auth2fa.NewVerifier(src, crypt.NewSecretGenerator(10), totp.NewAuthenticator("TestIssuer", 20))

	ok, commit, err := v.Verify(context.Background(), uuid.New(), confirmmethod.TOTP, "000000")
	require.ErrorIs(t, err, wantErr)
	require.False(t, ok)
	require.Nil(t, commit)
}
