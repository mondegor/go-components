package secondfactor_test

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
	"github.com/mondegor/go-components/mrauth/service/secondfactor"
)

const testTOTPSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

type fakeSource struct {
	row           entity.Auth2fa
	fetchErr      error
	consumeErr    error
	consumedHash  string
	consumeCalled bool
}

func (s *fakeSource) FetchOne(_ context.Context, _ uuid.UUID) (entity.Auth2fa, error) {
	if s.fetchErr != nil {
		return entity.Auth2fa{}, s.fetchErr
	}

	return s.row, nil
}

func (s *fakeSource) ConsumeRecoveryCode(_ context.Context, _ uuid.UUID, hash string) error {
	s.consumeCalled = true
	s.consumedHash = hash

	return s.consumeErr
}

// countingComparer - счётчик вызовов сравнения для проверки, что bcrypt не перебирается зря.
type countingComparer struct {
	calls int
}

func (c *countingComparer) CompareSecretAndHash(_, _ string) error {
	c.calls++

	return errors.New("no match")
}

func TestVerifier_ValidTOTP(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	src := &fakeSource{row: entity.Auth2fa{UserID: userID, Type: auth2fatype.TOTP, Secret: testTOTPSecret}}

	auth := totp.NewAuthenticator("TestIssuer", 20)

	code, err := auth.GenerateCode(testTOTPSecret, time.Now())
	require.NoError(t, err)

	v := secondfactor.NewVerifier(src, crypt.NewSecretGenerator(10), auth)

	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.TOTP, code)
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, commit)
	require.False(t, src.consumeCalled)
}

func TestVerifier_RecoveryFallbackConsumes(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	gen := crypt.NewSecretGenerator(10)

	h1, err := gen.HashedSecret("AAAAABBBBB")
	require.NoError(t, err)

	h2, err := gen.HashedSecret("CCCCCDDDDD")
	require.NoError(t, err)

	src := &fakeSource{row: entity.Auth2fa{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1, h2},
	}}

	v := secondfactor.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

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

	src := &fakeSource{row: entity.Auth2fa{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	}}

	v := secondfactor.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

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

	src := &fakeSource{row: entity.Auth2fa{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{"hash-1", "hash-2", "hash-3"},
	}}

	v := secondfactor.NewVerifier(src, comparer, totp.NewAuthenticator("TestIssuer", 20))

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

	// код уже израсходован параллельной операцией: ConsumeRecoveryCode возвращает ошибку
	src := &fakeSource{
		row: entity.Auth2fa{
			UserID:        userID,
			Type:          auth2fatype.TOTP,
			Secret:        testTOTPSecret,
			RecoveryCodes: []string{h1},
		},
		consumeErr: errors.New("record not found"),
	}

	v := secondfactor.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

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

	src := &fakeSource{row: entity.Auth2fa{UserID: userID, Type: auth2fatype.Password, Secret: hash}}

	v := secondfactor.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

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

	src := &fakeSource{row: entity.Auth2fa{UserID: userID, Type: auth2fatype.Password, Secret: hash}}

	v := secondfactor.NewVerifier(src, gen, totp.NewAuthenticator("TestIssuer", 20))

	ok, commit, err := v.Verify(context.Background(), userID, confirmmethod.Password, "wrong-password")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, commit)
}

func TestVerifier_FetchError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fetch failed")
	src := &fakeSource{fetchErr: wantErr}

	v := secondfactor.NewVerifier(src, crypt.NewSecretGenerator(10), totp.NewAuthenticator("TestIssuer", 20))

	ok, commit, err := v.Verify(context.Background(), uuid.New(), confirmmethod.TOTP, "000000")
	require.ErrorIs(t, err, wantErr)
	require.False(t, ok)
	require.Nil(t, commit)
}
