package auth2fa_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
	"github.com/mondegor/go-components/mrauth/service/auth2fa"
	"github.com/mondegor/go-components/mrauth/service/auth2fa/mock"
)

//go:generate mockgen -source=verifier.go -destination=mock/verifier.go -package=mock

const testTOTPSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

type VerifierSuite struct {
	suite.Suite

	ctrl        *gomock.Controller
	ctx         context.Context
	source      *mock.Mockuser2faSource
	alerter     *mock.MockrecoveryAlerter
	securityLog *mock.MocksecurityLogStorage
	userID      uuid.UUID

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
	s.securityLog = mock.NewMocksecurityLogStorage(s.ctrl)
	s.userID = uuid.New()
	s.gen = crypt.NewSecretGenerator()
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

// expectSecurityLog - ожидает запись траты аварийного кода в журнал безопасности
// (содержимое записи проверяет TestRecoveryConsumedLogsSecurityEvent).
func (s *VerifierSuite) expectSecurityLog() *gomock.Call {
	return s.securityLog.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)
}

func (s *VerifierSuite) newVerifier(opts ...auth2fa.Option) *auth2fa.Verifier {
	return auth2fa.NewVerifier(s.source, s.gen, s.auth, s.securityLog, opts...)
}

func (s *VerifierSuite) TestValidTOTP() {
	code, err := s.auth.GenerateCode(testTOTPSecret, time.Now())
	s.Require().NoError(err)

	s.expectFetch(entity.Auth2FA{Type: auth2fatype.TOTP, Secret: testTOTPSecret})
	// шаг фиксируется только при вызове commit
	s.source.EXPECT().UpdateTOTPStep(gomock.Any(), s.userID, gomock.Not(gomock.Eq(int64(0)))).Return(nil)

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, true, code)
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

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, true, code)
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestRecoveryFallbackConsumes() {
	h1, h2 := s.hashed("AAAAAAAA-BBBBBBBB"), s.hashed("CCCCCCCC-DDDDDDDD")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1, h2},
	})
	// израсходован именно совпавший хеш, и только после фиксации
	s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h1).Return(1, nil)
	s.expectSecurityLog()

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, true, "AAAAAAAA-BBBBBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)

	s.Require().NotNil(commit)

	s.Require().NoError(commit(s.ctx))
}

// TestRecoveryFallbackSurvivesFactorTypeChange - тип второго фактора сменился между созданием
// операции и её подтверждением: звено собрано как PASSWORD, а у аккаунта теперь TOTP. Основного
// доказательства у звена больше нет, но аварийный код от типа фактора не зависит и обязан быть
// принят - его приём обещан контрактом (см. contracts/mrauth/paths/v1_signin.yaml).
func (s *VerifierSuite) TestRecoveryFallbackSurvivesFactorTypeChange() {
	h1 := s.hashed("AAAAAAAA-BBBBBBBB")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	})
	s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h1).Return(1, nil)
	s.expectSecurityLog()

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Password, true, "AAAAAAAA-BBBBBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)

	s.Require().NotNil(commit)

	s.Require().NoError(commit(s.ctx))
}

func (s *VerifierSuite) TestInvalidTOTPNoRecoveryMatch() {
	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{s.hashed("AAAAAAAA-BBBBBBBB")},
	})

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, true, "ZZZZZZZZ-YYYYYYYY")
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
	// неверный код в формате TOTP (только цифры) не должен запускать сверку с хешами аварийных кодов
	comparer.EXPECT().CompareSecretAndHash(gomock.Any(), gomock.Any()).Times(0)

	v := auth2fa.NewVerifier(s.source, comparer, s.auth, s.securityLog)

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, true, "000000")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

// TestRecoveryWithoutSeparatorSkipsHashes - код подходящей длины, но без разделителя посередине
// не имеет формата аварийного кода: на звене аварийного кода он отклоняется без сверки с хешами.
func (s *VerifierSuite) TestRecoveryWithoutSeparatorSkipsHashes() {
	comparer := mock.NewMockpasswordComparer(s.ctrl)

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{"hash-1", "hash-2", "hash-3"},
	})
	comparer.EXPECT().CompareSecretAndHash(gomock.Any(), gomock.Any()).Times(0)

	v := auth2fa.NewVerifier(s.source, comparer, s.auth, s.securityLog)

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Recovery, false, "AAAAAAAABBBBBBBBB")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

// TestRecoveryNotAllowedSkipsRecovery - комбинация с аварийным кодом недопустима для текущего
// действия операции (например, перевыпуск аварийных кодов). Код в формате аварийного проверяется
// как обычное доказательство, а с хешами аварийных кодов не сравнивается: иначе он расходовался бы
// на операции, которая его не принимает.
func (s *VerifierSuite) TestRecoveryNotAllowedSkipsRecovery() {
	comparer := mock.NewMockpasswordComparer(s.ctrl)

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{"hash-1", "hash-2", "hash-3"},
	})
	comparer.EXPECT().CompareSecretAndHash(gomock.Any(), gomock.Any()).Times(0)

	v := auth2fa.NewVerifier(s.source, comparer, s.auth, s.securityLog)

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, false, "AAAAAAAA-BBBBBBBB")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

// TestRecoveryCodeLengthBelowSeparatorNormalized - нижняя граница длины короче кода
// с разделителем поднимается до неё, а верхняя подтягивается к нижней: код минимальной
// длины с разделителем принимается.
func (s *VerifierSuite) TestRecoveryCodeLengthBelowSeparatorNormalized() {
	h1 := s.hashed("AAAAA-BBBBB")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	})

	v := s.newVerifier(auth2fa.WithRecoveryCodeLength(5, 5))

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Recovery, false, "AAAAA-BBBBB")
	s.Require().NoError(err)
	s.True(ok)
	s.NotNil(commit)
}

func (s *VerifierSuite) TestVerifyRecoveryConsumes() {
	h1, h2 := s.hashed("AAAAAAAA-BBBBBBBB"), s.hashed("CCCCCCCC-DDDDDDDD")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1, h2},
	})
	s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h2).Return(1, nil)
	s.expectSecurityLog()

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Recovery, false, "CCCCCCCC-DDDDDDDD")
	s.Require().NoError(err)
	s.Require().True(ok)

	s.Require().NotNil(commit)

	s.Require().NoError(commit(s.ctx))
}

func (s *VerifierSuite) TestVerifyRecoveryNoMatch() {
	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{s.hashed("AAAAAAAA-BBBBBBBB")},
	})

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Recovery, false, "ZZZZZZZZ-YYYYYYYY")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

// TestVerifyRecoverySkipsShortCode - код, не похожий на аварийный (обычный код с емаила),
// отклоняется завершающим звеном без сверки с хешами аварийных кодов.
func (s *VerifierSuite) TestVerifyRecoverySkipsShortCode() {
	comparer := mock.NewMockpasswordComparer(s.ctrl)

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{"hash-1", "hash-2", "hash-3"},
	})
	comparer.EXPECT().CompareSecretAndHash(gomock.Any(), gomock.Any()).Times(0)

	v := auth2fa.NewVerifier(s.source, comparer, s.auth, s.securityLog)

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Recovery, false, "183947")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

// TestVerifyRecovery2FADisabled - записи 2FA нет: её отключили между подтверждением
// предыдущего звена и предъявлением аварийного кода. Отказ отдаётся как есть, скрывать его
// незачем: до завершающего звена доходит только тот, кто уже подтвердил второй фактор,
// то есть про включённую 2FA он и так знает.
func (s *VerifierSuite) TestVerifyRecovery2FADisabled() {
	s.source.EXPECT().
		FetchOne(gomock.Any(), s.userID).
		Return(entity.Auth2FA{}, sysmesserrors.ErrEventStorageNoRecordFound)

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Recovery, false, "AAAAAAAA-BBBBBBBB")
	s.Require().ErrorIs(err, mrauth.ErrAuth2FAIsDisabled)
	s.False(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestVerifyRecoveryFetchError() {
	wantErr := errors.New("fetch failed")
	s.source.EXPECT().FetchOne(gomock.Any(), s.userID).Return(entity.Auth2FA{}, wantErr)

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Recovery, false, "AAAAAAAA-BBBBBBBB")
	s.Require().ErrorIs(err, wantErr)
	s.False(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestRecoveryConsumeRace() {
	h1 := s.hashed("AAAAAAAA-BBBBBBBB")
	consumeErr := errors.New("record not found")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	})
	// код уже израсходован параллельной операцией
	s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h1).Return(0, consumeErr)

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, true, "AAAAAAAA-BBBBBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	s.Require().ErrorIs(commit(s.ctx), consumeErr)
}

func (s *VerifierSuite) TestPasswordCorrect() {
	s.expectFetch(entity.Auth2FA{Type: auth2fatype.Password, Secret: s.hashed("my-secret-password")})

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Password, true, "my-secret-password")
	s.Require().NoError(err)
	s.True(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestPasswordWrong() {
	s.expectFetch(entity.Auth2FA{Type: auth2fatype.Password, Secret: s.hashed("my-secret-password")})

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Password, true, "wrong-password")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestPasswordRecoveryFallbackConsumes() {
	recHash := s.hashed("AAAAAAAA-BBBBBBBB")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.Password,
		Secret:        s.hashed("my-secret-password"),
		RecoveryCodes: []string{recHash},
	})
	s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, recHash).Return(1, nil)
	s.expectSecurityLog()

	// пароль не подошёл, но предъявлен валидный аварийный код - он засчитывается и расходуется
	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Password, true, "AAAAAAAA-BBBBBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)

	s.Require().NotNil(commit)

	s.Require().NoError(commit(s.ctx))
}

func (s *VerifierSuite) TestPasswordWrongNoRecoveryMatch() {
	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.Password,
		Secret:        s.hashed("my-secret-password"),
		RecoveryCodes: []string{s.hashed("AAAAAAAA-BBBBBBBB")},
	})

	// ни пароль, ни аварийный код не совпали - доступ не предоставляется, код не расходуется
	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Password, true, "ZZZZZZZZ-YYYYYYYY")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

func (s *VerifierSuite) TestRecoveryConsumedCallsAlerter() {
	h1 := s.hashed("AAAAAAAA-BBBBBBBB")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	})
	// Verifier сообщает alerter'у остаток на каждый израсходованный код вместе с клиентом,
	// предъявившим код. Обе фиксации происходят только после commit, поэтому порядок задан явно.
	actor := dto.ActorMeta{UserID: s.userID, UserAgent: "test-agent"}

	gomock.InOrder(
		s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h1).Return(1, nil),
		s.expectSecurityLog(),
		s.alerter.EXPECT().SendAlert(gomock.Any(), actor, 1).Return(nil),
	)

	v := s.newVerifier(auth2fa.WithRecoveryAlerter(s.alerter))

	ok, commit, err := v.Verify(s.ctx, actor, confirmmethod.TOTP, true, "AAAAAAAA-BBBBBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	s.Require().NoError(commit(s.ctx))
}

// TestRecoveryConsumedLogsSecurityEvent - трата аварийного кода записывается в журнал
// безопасности пользователя с остатком кодов, до оповещения и в той же фиксации.
func (s *VerifierSuite) TestRecoveryConsumedLogsSecurityEvent() {
	h1 := s.hashed("AAAAAAAA-BBBBBBBB")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	})

	actor := dto.ActorMeta{UserID: s.userID, UserAgent: "test-agent"}
	remaining := 4

	gomock.InOrder(
		s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h1).Return(remaining, nil),
		s.securityLog.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, row entity.SecurityLogEvent) error {
				s.Equal(s.userID, row.UserID)
				s.Equal("test-agent", row.UserAgent)
				s.Equal(securityevent.RecoveryCodeUsed, row.EventType)
				s.Equal(&entity.SecurityLogExtra{Remaining: &remaining}, row.Extra)

				return nil
			},
		),
		s.alerter.EXPECT().SendAlert(gomock.Any(), actor, remaining).Return(nil),
	)

	v := s.newVerifier(auth2fa.WithRecoveryAlerter(s.alerter))

	ok, commit, err := v.Verify(s.ctx, actor, confirmmethod.TOTP, true, "AAAAAAAA-BBBBBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	s.Require().NoError(commit(s.ctx))
}

// TestSecurityLogErrorFailsCommit - сбой записи в журнал безопасности проваливает фиксацию:
// подтверждение откатывается вместе с гашением кода, оповещение не отправляется
// (мок alerter'а без EXPECT: любой вызов провалит тест).
func (s *VerifierSuite) TestSecurityLogErrorFailsCommit() {
	h1 := s.hashed("AAAAAAAA-BBBBBBBB")
	wantErr := errors.New("insert failed")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	})
	s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h1).Return(1, nil)
	s.securityLog.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(wantErr)

	v := s.newVerifier(auth2fa.WithRecoveryAlerter(s.alerter))

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, true, "AAAAAAAA-BBBBBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	s.Require().ErrorIs(commit(s.ctx), wantErr)
}

func (s *VerifierSuite) TestFetchError() {
	wantErr := errors.New("fetch failed")
	s.source.EXPECT().FetchOne(gomock.Any(), gomock.Any()).Return(entity.Auth2FA{}, wantErr)

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: uuid.New()}, confirmmethod.TOTP, true, "000000")
	s.Require().ErrorIs(err, wantErr)
	s.False(ok)
	s.Nil(commit)
}

// TestFetch2FADisabledIsReported - записи 2FA нет: у аккаунта её либо никогда не было
// (цепочка-заглушка входа по аварийному коду), либо её удалили между созданием операции
// и её подтверждением. Верификатор отдаёт этот факт как есть, а скрывать ли состояние
// аккаунта - решает вызывающий.
func (s *VerifierSuite) TestFetch2FADisabledIsReported() {
	s.source.EXPECT().
		FetchOne(gomock.Any(), gomock.Any()).
		Return(entity.Auth2FA{}, sysmesserrors.ErrEventStorageNoRecordFound)

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: uuid.New()}, confirmmethod.TOTP, true, "000000")
	s.Require().ErrorIs(err, mrauth.ErrAuth2FAIsDisabled)
	s.False(ok)
	s.Nil(commit)
}

// TestFetch2FADisabledStillComparesSecret - главное свойство заглушки: сверка выполняется
// даже когда сверять не с чем. Пропусти её верификатор - аккаунт без 2FA ответил бы заметно
// быстрее аккаунта с 2FA и неверным паролем, и состояние аккаунта читалось бы по времени ответа.
func (s *VerifierSuite) TestFetch2FADisabledStillComparesSecret() {
	comparer := mock.NewMockpasswordComparer(s.ctrl)

	s.source.EXPECT().
		FetchOne(gomock.Any(), gomock.Any()).
		Return(entity.Auth2FA{}, sysmesserrors.ErrEventStorageNoRecordFound)
	// сверка идёт с подставным хешем, а не с пустой строкой: сверка с пустым хешем не обязана
	// стоить столько же, сколько с настоящим, и выдала бы состояние аккаунта
	comparer.EXPECT().
		CompareSecretAndHash("any-password", gomock.Not(gomock.Eq(""))).
		Return(false, nil).
		Times(1)

	v := auth2fa.NewVerifier(s.source, comparer, s.auth, s.securityLog)

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: uuid.New()}, confirmmethod.Password, true, "any-password")
	s.Require().ErrorIs(err, mrauth.ErrAuth2FAIsDisabled)
	s.False(ok)
	s.Nil(commit)
}

// TestFactorTypeMismatchStillComparesSecret - тип фактора сменился после создания операции:
// основного доказательства у звена нет, но сверка с подставным секретом всё равно выполняется,
// иначе ответ пришёл бы быстрее, чем на неверное значение фактора.
func (s *VerifierSuite) TestFactorTypeMismatchStillComparesSecret() {
	comparer := mock.NewMockpasswordComparer(s.ctrl)

	s.expectFetch(entity.Auth2FA{Type: auth2fatype.TOTP, Secret: testTOTPSecret})
	comparer.EXPECT().
		CompareSecretAndHash("any-password", gomock.Not(gomock.Eq(""))).
		Return(false, nil).
		Times(1)

	v := auth2fa.NewVerifier(s.source, comparer, s.auth, s.securityLog)

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.Password, false, "any-password")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

// TestFactorTypeMismatchStillValidatesTOTP - зеркальный случай к тесту выше: звено ждёт TOTP,
// а у аккаунта теперь пароль; код сверяется с подставным TOTP-секретом, а не с хешем пароля.
func (s *VerifierSuite) TestFactorTypeMismatchStillValidatesTOTP() {
	validator := mock.NewMocktotpValidator(s.ctrl)
	passwordHash := s.hashed("real-password")

	s.expectFetch(entity.Auth2FA{Type: auth2fatype.Password, Secret: passwordHash})
	validator.EXPECT().
		ValidateCode("000000", gomock.Not(gomock.Eq(passwordHash))).
		Return(false, int64(0), nil).
		Times(1)

	v := auth2fa.NewVerifier(s.source, s.gen, validator, s.securityLog)

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, false, "000000")
	s.Require().NoError(err)
	s.False(ok)
	s.Nil(commit)
}

// TestFetch2FADisabledDecoyErrorIsSwallowed - ошибка подставной сверки наружу не выходит:
// вызывающий выдаёт этот ответ за неверное доказательство, и всплывшая ошибка сверки была бы
// ровно тем признаком, который заглушка и скрывает.
func (s *VerifierSuite) TestFetch2FADisabledDecoyErrorIsSwallowed() {
	comparer := mock.NewMockpasswordComparer(s.ctrl)

	s.source.EXPECT().
		FetchOne(gomock.Any(), gomock.Any()).
		Return(entity.Auth2FA{}, sysmesserrors.ErrEventStorageNoRecordFound)
	comparer.EXPECT().
		CompareSecretAndHash(gomock.Any(), gomock.Any()).
		Return(false, errors.New("broken decoy hash"))

	v := auth2fa.NewVerifier(s.source, comparer, s.auth, s.securityLog)

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: uuid.New()}, confirmmethod.Password, true, "any-password")
	s.Require().ErrorIs(err, mrauth.ErrAuth2FAIsDisabled)
	s.Require().NotContains(err.Error(), "broken decoy hash")
	s.False(ok)
	s.Nil(commit)
}

// TestDecoyPasswordHashIsWellFormed - подставной хеш обязан быть настоящим bcrypt-хешем той же
// стоимости, что и хеши паролей. Битый хеш verifyDecoy молча проглотит (см. тест выше), сверка
// завершится мгновенно, и выравнивание времени ответа S0/S1 перестанет работать незаметно.
func (s *VerifierSuite) TestDecoyPasswordHashIsWellFormed() {
	comparer := mock.NewMockpasswordComparer(s.ctrl)

	var decoyHash string

	s.source.EXPECT().
		FetchOne(gomock.Any(), gomock.Any()).
		Return(entity.Auth2FA{}, sysmesserrors.ErrEventStorageNoRecordFound)
	comparer.EXPECT().
		CompareSecretAndHash(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_, hashedSecret string) (bool, error) {
			decoyHash = hashedSecret

			return false, nil
		})

	v := auth2fa.NewVerifier(s.source, comparer, s.auth, s.securityLog)

	_, _, err := v.Verify(s.ctx, dto.ActorMeta{UserID: uuid.New()}, confirmmethod.Password, true, "any-password")
	s.Require().ErrorIs(err, mrauth.ErrAuth2FAIsDisabled)

	// хеш проверяется настоящим генератором: сверка обязана не сойтись и не дать ошибки
	ok, err := s.gen.CompareSecretAndHash("any-password", decoyHash)
	s.Require().NoError(err)
	s.False(ok)
}

// TestDecoyTOTPSecretIsWellFormed - то же требование к подставному TOTP-секрету: он обязан
// декодироваться как base32, иначе проверка кода завершится ошибкой вместо полноценной сверки.
func (s *VerifierSuite) TestDecoyTOTPSecretIsWellFormed() {
	validator := mock.NewMocktotpValidator(s.ctrl)

	var decoySecret string

	s.source.EXPECT().
		FetchOne(gomock.Any(), gomock.Any()).
		Return(entity.Auth2FA{}, sysmesserrors.ErrEventStorageNoRecordFound)
	validator.EXPECT().
		ValidateCode(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_, secret string) (bool, int64, error) {
			decoySecret = secret

			return false, 0, nil
		})

	v := auth2fa.NewVerifier(s.source, s.gen, validator, s.securityLog)

	_, _, err := v.Verify(s.ctx, dto.ActorMeta{UserID: uuid.New()}, confirmmethod.TOTP, true, "000000")
	s.Require().ErrorIs(err, mrauth.ErrAuth2FAIsDisabled)

	// секрет проверяется настоящим аутентификатором: код по нему обязан считаться без ошибки
	_, err = s.auth.GenerateCode(decoySecret, time.Now())
	s.Require().NoError(err)
}

// TestCommitTOTPStepRaceTranslated - TOTP-шаг не удалось продвинуть: тот же time-step уже
// израсходован конкурентным подтверждением. Верификатор обязан перевести "запись не найдена"
// в доменный сигнал прямо здесь: только он знает, какой запрос её вернул.
func (s *VerifierSuite) TestCommitTOTPStepRaceTranslated() {
	code, err := s.auth.GenerateCode(testTOTPSecret, time.Now())
	s.Require().NoError(err)

	s.expectFetch(entity.Auth2FA{Type: auth2fatype.TOTP, Secret: testTOTPSecret})
	s.source.EXPECT().
		UpdateTOTPStep(gomock.Any(), s.userID, gomock.Any()).
		Return(sysmesserrors.ErrEventStorageNoRecordFound)

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, true, code)
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	commitErr := commit(s.ctx)
	s.Require().ErrorIs(commitErr, mrauth.ErrEventAuth2FACodeAlreadyUsed)
	s.Require().NotErrorIs(commitErr, sysmesserrors.ErrEventStorageNoRecordFound)
}

// TestCommitRecoveryCodeRaceTranslated - аварийный код израсходован конкурентным
// подтверждением: тот же перевод, что и для TOTP-шага.
func (s *VerifierSuite) TestCommitRecoveryCodeRaceTranslated() {
	h1 := s.hashed("AAAAAAAA-BBBBBBBB")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	})
	s.source.EXPECT().
		UpdateRecoveryCode(gomock.Any(), s.userID, h1).
		Return(0, sysmesserrors.ErrEventStorageNoRecordFound)

	ok, commit, err := s.newVerifier().Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, true, "AAAAAAAA-BBBBBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	commitErr := commit(s.ctx)
	s.Require().ErrorIs(commitErr, mrauth.ErrEventAuth2FACodeAlreadyUsed)
	s.Require().NotErrorIs(commitErr, sysmesserrors.ErrEventStorageNoRecordFound)
}

// TestAlerterErrorIsNotTranslated - код успешно израсходован, а alerter вернул ошибку. Любая
// его ошибка, в том числе "запись не найдена", к расходу кода отношения не имеет и обязана
// дойти как есть: иначе вызывающий код примет сбой alerter'а за повтор второго
// фактора и отдаст клиенту "неверный код" вместо внутренней ошибки.
func (s *VerifierSuite) TestAlerterErrorIsNotTranslated() {
	h1 := s.hashed("AAAAAAAA-BBBBBBBB")

	s.expectFetch(entity.Auth2FA{
		Type:          auth2fatype.TOTP,
		Secret:        testTOTPSecret,
		RecoveryCodes: []string{h1},
	})
	gomock.InOrder(
		s.source.EXPECT().UpdateRecoveryCode(gomock.Any(), s.userID, h1).Return(1, nil),
		s.expectSecurityLog(),
		s.alerter.EXPECT().
			SendAlert(gomock.Any(), gomock.Any(), 1).
			Return(sysmesserrors.ErrEventStorageNoRecordFound),
	)

	v := s.newVerifier(auth2fa.WithRecoveryAlerter(s.alerter))

	ok, commit, err := v.Verify(s.ctx, dto.ActorMeta{UserID: s.userID}, confirmmethod.TOTP, true, "AAAAAAAA-BBBBBBBB")
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(commit)

	commitErr := commit(s.ctx)
	s.Require().ErrorIs(commitErr, sysmesserrors.ErrEventStorageNoRecordFound)
	s.Require().NotErrorIs(commitErr, mrauth.ErrEventAuth2FACodeAlreadyUsed)
}
