package auth2fa_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/service/auth2fa"
	"github.com/mondegor/go-components/mrauth/service/auth2fa/mock"
)

//go:generate mockgen -source=verifier.go -destination=mock/verifier.go -package=mock
//go:generate mockgen -destination=mock/mrnotifier.go -package=mock github.com/mondegor/go-components/mrnotifier NoteProducer

const testTOTPSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

type VerifierSuite struct {
	suite.Suite

	ctrl    *gomock.Controller
	ctx     context.Context
	source  *mock.Mockuser2faSource
	alerter *mock.MockrecoveryAlerter
	userID  uuid.UUID

	// gen и auth - настоящие реализации: политика хеширования и проверки TOTP
	// проверяется вместе с верификатором, а не подменяется.
	gen  *crypt.SecretGenerator
	auth *totp.Authenticator
}

func TestVerifierSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(VerifierSuite))
}

func (s *VerifierSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.source = mock.NewMockuser2faSource(s.ctrl)
	s.alerter = mock.NewMockrecoveryAlerter(s.ctrl)
	s.userID = uuid.New()
	s.gen = crypt.NewSecretGenerator(10)
	s.auth = totp.NewAuthenticator("TestIssuer", 20)
}

// expectFetch - отдаёт верификатору указанную запись 2FA пользователя.
func (s *VerifierSuite) expectFetch(row entity.Auth2FA) {
	row.UserID = s.userID
	s.source.EXPECT().FetchOne(gomock.Any(), s.userID).Return(row, nil)
}

// hashed - хеш секрета, пригодный для сравнения настоящим генератором.
func (s *VerifierSuite) hashed(secret string) string {
	hash, err := s.gen.HashedSecret(secret)
	s.Require().NoError(err)

	return hash
}

func (s *VerifierSuite) newVerifier(opts ...auth2fa.Option) *auth2fa.Verifier {
	return auth2fa.NewVerifier(s.source, s.gen, s.auth, opts...)
}

func (s *VerifierSuite) TestValidTOTP() {
	code, err := s.auth.GenerateCode(testTOTPSecret, time.Now())
	s.Require().NoError(err)

	s.expectFetch(entity.Auth2FA{Type: auth2fatype.TOTP, Secret: testTOTPSecret})
	// шаг фиксируется только при вызове commit
	s.source.EXPECT().UpdateTOTPStep(gomock.Any(), s.userID, gomock.Not(gomock.Eq(int64(0)))).Return(nil)

	ok, commit, err := s.newVerifier().Verify(s.ctx, s.userID, confirmmethod.TOTP, code)
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit) // успешный TOTP возвращает commit для фиксации использованного шага

	s.Require().NoError(commit(s.ctx))
}

func (s *VerifierSuite) TestTOTPReplayRejected() {
	now := time.Now()

	code, err := s.auth.GenerateCode(testTOTPSecret, now)
	s.Require().NoError(err)

	// последний использованный шаг заведомо не меньше текущего: код того же окна
	// должен быть отклонён как повтор (replay).
	s.expectFetch(entity.Auth2FA{
		Type:         auth2fatype.TOTP,
		Secret:       testTOTPSecret,
		LastTOTPStep: now.Unix()/30 + 5,
	})

	ok, commit, err := s.newVerifier().Verify(s.ctx, s.userID, confirmmethod.TOTP, code)
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestRecoveryFallbackConsumes() {
	h1, h2 := s.hashed("AAAAABBBBB"), s.hashed("CCCCCDDDDD")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1, h2},
	})
	// израсходован именно совпавший хеш, и только после фиксации
	s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h1).Return(1, nil)

	ok, commit, err := s.newVerifier().Verify(s.ctx, s.userID, confirmmethod.TOTP, "AAAAABBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	s.Require().NoError(commit(s.ctx))
}

func (s *VerifierSuite) TestInvalidTOTPNoRecoveryMatch() {
	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{s.hashed("AAAAABBBBB")},
	})

	ok, commit, err := s.newVerifier().Verify(s.ctx, s.userID, confirmmethod.TOTP, "ZZZZZYYYYY")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestAllDigitCodeSkipsRecovery() {
	comparer := mock.NewMockpasswordComparer(s.ctrl)

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{"hash-1", "hash-2", "hash-3"},
	})
	// неверный код в формате TOTP (только цифры) не должен запускать перебор bcrypt-хешей
	comparer.EXPECT().CompareSecretAndHash(gomock.Any(), gomock.Any()).Times(0)

	v := auth2fa.NewVerifier(s.source, comparer, s.auth)

	ok, commit, err := v.Verify(s.ctx, s.userID, confirmmethod.TOTP, "000000")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestRecoveryConsumeRace() {
	h1 := s.hashed("AAAAABBBBB")
	consumeErr := errors.New("record not found")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	})
	// код уже израсходован параллельной операцией
	s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h1).Return(0, consumeErr)

	ok, commit, err := s.newVerifier().Verify(s.ctx, s.userID, confirmmethod.TOTP, "AAAAABBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	s.Require().ErrorIs(commit(s.ctx), consumeErr)
}

func (s *VerifierSuite) TestPasswordCorrect() {
	s.expectFetch(entity.Auth2FA{Type: auth2fatype.Password, Secret: s.hashed("my-secret-password")})

	ok, commit, err := s.newVerifier().Verify(s.ctx, s.userID, confirmmethod.Password, "my-secret-password")
	s.Require().NoError(err)
	s.True(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestPasswordWrong() {
	s.expectFetch(entity.Auth2FA{Type: auth2fatype.Password, Secret: s.hashed("my-secret-password")})

	ok, commit, err := s.newVerifier().Verify(s.ctx, s.userID, confirmmethod.Password, "wrong-password")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestPasswordRecoveryFallbackConsumes() {
	recHash := s.hashed("AAAAABBBBB")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.Password,
		Secret:        s.hashed("my-secret-password"),
		RecoveryCodes: []string{recHash},
	})
	s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, recHash).Return(1, nil)

	// пароль не подошёл, но предъявлен валидный аварийный код - он засчитывается и расходуется
	ok, commit, err := s.newVerifier().Verify(s.ctx, s.userID, confirmmethod.Password, "AAAAABBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	s.Require().NoError(commit(s.ctx))
}

func (s *VerifierSuite) TestPasswordWrongNoRecoveryMatch() {
	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.Password,
		Secret:        s.hashed("my-secret-password"),
		RecoveryCodes: []string{s.hashed("AAAAABBBBB")},
	})

	// ни пароль, ни аварийный код не совпали - доступ не предоставляется, код не расходуется
	ok, commit, err := s.newVerifier().Verify(s.ctx, s.userID, confirmmethod.Password, "ZZZZZYYYYY")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestRecoveryConsumedCallsAlerter() {
	h1 := s.hashed("AAAAABBBBB")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	})
	// Verifier всегда сообщает остаток alerter'у; решение о пороге - на стороне alerter.
	// Обе фиксации происходят только после commit, поэтому порядок задан явно.
	gomock.InOrder(
		s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h1).Return(1, nil),
		s.alerter.EXPECT().SendAlert(gomock.Any(), s.userID, 1).Return(nil),
	)

	v := s.newVerifier(auth2fa.WithRecoveryAlerter(s.alerter))

	ok, commit, err := v.Verify(s.ctx, s.userID, confirmmethod.TOTP, "AAAAABBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	s.Require().NoError(commit(s.ctx))
}

func (s *VerifierSuite) TestFetchError() {
	wantErr := errors.New("fetch failed")
	s.source.EXPECT().FetchOne(gomock.Any(), gomock.Any()).Return(entity.Auth2FA{}, wantErr)

	ok, commit, err := s.newVerifier().Verify(s.ctx, uuid.New(), confirmmethod.TOTP, "000000")
	s.Require().ErrorIs(err, wantErr)
	s.False(ok)
	s.Nil(commit)
}
