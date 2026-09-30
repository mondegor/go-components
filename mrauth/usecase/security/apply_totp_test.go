package security_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/service/notify"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager

// testTotpSecret - валидный base32 TOTP-secret, используемый в тестах verify_totp.
const testTotpSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

// baseSuite - общие для пакета моки (транзакция, уведомления, журнал операций)
// и накопленные записи журнала; встраивается наборами всех файлов пакета.
type baseSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	ctx          context.Context
	txManager    *mock.MockDBTxManager
	notifierAPI  *mock.MockNotifier
	logOperation *mock.MockoperationLogger
	logEntries   []entity.SecureOperationLog
	actorProps   *notify.ActorProps
	notified     bool
	notifiedKey  string
	notifiedWith map[string]any
}

func (s *baseSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.notifierAPI = mock.NewMockNotifier(s.ctrl)
	s.logOperation = mock.NewMockoperationLogger(s.ctrl)
	s.logEntries = nil
	s.actorProps = notify.NewActorProps(func(string) (string, string) {
		return "TestApp", "TestDevice"
	})
	s.notified = false
	s.notifiedKey = ""
	s.notifiedWith = nil

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()

	s.notifierAPI.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key string, props map[string]any) error {
			s.notified = true
			s.notifiedKey = key
			s.notifiedWith = props

			return nil
		}).
		AnyTimes()

	s.logOperation.EXPECT().
		Log(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.SecureOperationLog) {
			s.logEntries = append(s.logEntries, entry)
		}).
		AnyTimes()
}

func (s *baseSuite) SetupSubTest() {
	s.SetupTest()
}

type ApplyTOTPSuite struct {
	baseSuite

	binder   *mock.Mockuser2faBinder
	verifier *mock.MockoperationDeleter
	revoker  *mock.MockoperationRevoker
	saved    entity.Auth2FA
	deleted  string
	bindErr  error // ошибка, которую вернёт привязка 2FA (по умолчанию привязка успешна)

	revokedFor   uuid.UUID      // пользователь, чьи операции отозваны (uuid.Nil - не отзывали)
	revokeReason logreason.Enum // причина отзыва
	revokeErr    error          // ошибка, которую вернёт отзыв операций
}

func TestApplyTOTPSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApplyTOTPSuite))
}

func (s *ApplyTOTPSuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.binder = mock.NewMockuser2faBinder(s.ctrl)
	s.verifier = mock.NewMockoperationDeleter(s.ctrl)
	s.revoker = mock.NewMockoperationRevoker(s.ctrl)
	s.saved = entity.Auth2FA{}
	s.deleted = ""
	s.bindErr = nil
	s.revokedFor = uuid.Nil
	s.revokeReason = logreason.Unspecified
	s.revokeErr = nil

	s.revoker.EXPECT().
		RevokeAll(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, actor dto.ActorMeta, reason logreason.Enum) error {
			s.revokedFor = actor.UserID
			s.revokeReason = reason

			return s.revokeErr
		}).
		AnyTimes()

	s.binder.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, row entity.Auth2FA) error {
			if s.bindErr != nil {
				return s.bindErr
			}

			s.saved = row

			return nil
		}).
		AnyTimes()

	s.verifier.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token string) error {
			s.deleted = token

			return nil
		}).
		AnyTimes()
}

func (s *ApplyTOTPSuite) TestValidCodeBindsAndReturnsCodes() {
	userID := uuid.New()
	op := confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`)

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)

	auth := totp.NewAuthenticator("TestIssuer", 20)
	uc := security.NewApplyTOTPGenerator(
		s.txManager, s.binder, s.verifier, s.revoker,
		crypt.NewSecretGenerator(), auth, s.notifierAPI, s.actorProps, s.logOperation, 10, 10,
	)

	code, err := auth.GenerateCode(testTotpSecret, time.Now())
	s.Require().NoError(err)

	codes, err := uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token", code)
	s.Require().NoError(err)
	s.Require().Len(codes, 10)
	s.Equal(auth2fatype.TOTP, s.saved.Type)
	s.Equal(testTotpSecret, s.saved.Secret)
	s.Require().Len(s.saved.RecoveryCodes, 10)
	s.NotEqual(codes, s.saved.RecoveryCodes) // хранятся хеши, возвращается plaintext
	s.Equal("op-token", s.deleted)
	s.True(s.notified)
	s.Equal("user.2fa.enabled", s.notifiedKey)
	s.Equal(auth2fatype.TOTP.String(), s.notifiedWith["factor"])
	// включение 2FA отзывает все незавершённые операции пользователя
	s.Equal(userID, s.revokedFor)
	s.Equal(logreason.Auth2FAStateChanged, s.revokeReason)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Applied, s.logEntries[0].LogStatus)
	s.Equal(operationtype.ChangeTOTP.String(), s.logEntries[0].SourceName)
}

// TestActive2FAConflictNoApply - 2FA включили другим способом между созданием операции
// и её применением: привязка отклоняется нарушением уникальности, операция остаётся
// неприменённой, а наружу уходит ErrAuth2FAMustBeDisabledFirst (409).
func (s *ApplyTOTPSuite) TestActive2FAConflictNoApply() {
	userID := uuid.New()
	op := confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`)
	s.bindErr = errors.ErrInternalStorageDuplicateKeyViolation.New()

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)

	auth := totp.NewAuthenticator("TestIssuer", 20)
	uc := security.NewApplyTOTPGenerator(
		s.txManager, s.binder, s.verifier, s.revoker,
		crypt.NewSecretGenerator(), auth, s.notifierAPI, s.actorProps, s.logOperation, 10, 10,
	)

	code, err := auth.GenerateCode(testTotpSecret, time.Now())
	s.Require().NoError(err)

	codes, err := uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token", code)
	s.Require().ErrorIs(err, mrauth.ErrAuth2FAMustBeDisabledFirst)
	s.Nil(codes)
	s.Equal(entity.Auth2FA{}, s.saved, "второй фактор не должен привязываться")
	s.Empty(s.deleted, "операция не должна применяться")
	s.Equal(uuid.Nil, s.revokedFor, "операции не отзываются: 2FA не включилась")
	s.False(s.notified)
	// гонка с включением 2FA другим способом фиксируется в журнале как блокировка
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.Auth2FAStateChanged, s.logEntries[0].Reason)
}

func (s *ApplyTOTPSuite) TestInvalidCodeNoBind() {
	userID := uuid.New()
	op := confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`)

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)

	uc := security.NewApplyTOTPGenerator(
		s.txManager, s.binder, s.verifier, s.revoker,
		crypt.NewSecretGenerator(), totp.NewAuthenticator("TestIssuer", 20),
		s.notifierAPI, s.actorProps, s.logOperation, 10, 10,
	)

	codes, err := uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token", "000000")
	s.Require().ErrorIs(err, mrauth.ErrTOTPCodeIsIncorrect)
	s.Nil(codes)
	s.Equal(entity.Auth2FA{}, s.saved)
	s.Empty(s.deleted)
	s.False(s.notified)
	// неверный TOTP-код - это неудачное подтверждение, а не блокировка
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.ConfirmFailed, s.logEntries[0].LogStatus)
	s.Equal(logreason.WrongCode, s.logEntries[0].Reason)
}
