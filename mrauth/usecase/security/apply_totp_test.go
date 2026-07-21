package security_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

//go:generate mockgen -source=apply_totp.go -destination=mock/apply_totp.go -package=mock
//go:generate mockgen -source=apply_operation.go -destination=mock/apply_operation.go -package=mock
//go:generate mockgen -source=apply_recovery.go -destination=mock/apply_recovery.go -package=mock
//go:generate mockgen -source=render_totp_qr.go -destination=mock/render_totp_qr.go -package=mock
//go:generate mockgen -source=change_email.go -destination=mock/change_email.go -package=mock
//go:generate mockgen -source=change_phone.go -destination=mock/change_phone.go -package=mock
//go:generate mockgen -source=change_totp.go -destination=mock/change_totp.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrnotifier.go -package=mock github.com/mondegor/go-components/mrnotifier NoteProducer
//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth User2FAConfirmActionCreator,OperationHandler

// testTotpSecret - валидный base32 TOTP-secret, используемый в тестах verify_totp.
const testTotpSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

// baseSuite - общие для пакета моки (транзакция, уведомления, журнал операций)
// и накопленные записи журнала; встраивается наборами всех файлов пакета.
type baseSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	ctx          context.Context
	txManager    *mock.MockDBTxManager
	notifierAPI  *mock.MockNoteProducer
	logOperation *mock.MockoperationLogger
	logEntries   []entity.SecureOperationLog
	notified     bool
}

func (s *baseSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.notifierAPI = mock.NewMockNoteProducer(s.ctrl)
	s.logOperation = mock.NewMockoperationLogger(s.ctrl)
	s.logEntries = nil
	s.notified = false

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()

	s.notifierAPI.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, string, map[string]any) error {
			s.notified = true

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
	saved    entity.Auth2FA
	deleted  string
}

func TestApplyTOTPSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApplyTOTPSuite))
}

func (s *ApplyTOTPSuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.binder = mock.NewMockuser2faBinder(s.ctrl)
	s.verifier = mock.NewMockoperationDeleter(s.ctrl)
	s.saved = entity.Auth2FA{}
	s.deleted = ""

	s.binder.EXPECT().
		InsertOrUpdate(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, row entity.Auth2FA) error {
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
		s.txManager, s.binder, s.verifier,
		crypt.NewSecretGenerator(10), auth, s.notifierAPI, s.logOperation, 10,
	)

	code, err := auth.GenerateCode(testTotpSecret, time.Now())
	s.Require().NoError(err)

	codes, err := uc.Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token", code)
	s.Require().NoError(err)
	s.Require().Len(codes, 10)
	s.Equal(auth2fatype.TOTP, s.saved.Type)
	s.Equal(testTotpSecret, s.saved.Secret)
	s.Require().Len(s.saved.RecoveryCodes, 10)
	s.NotEqual(codes, s.saved.RecoveryCodes) // хранятся хеши, возвращается plaintext
	s.Equal("op-token", s.deleted)
	s.True(s.notified)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Applied, s.logEntries[0].LogStatus)
	s.Equal(unit.NameConfirmChangeTOTP, s.logEntries[0].OperationName)
}

func (s *ApplyTOTPSuite) TestInvalidCodeNoBind() {
	userID := uuid.New()
	op := confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`)

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)

	uc := security.NewApplyTOTPGenerator(
		s.txManager, s.binder, s.verifier,
		crypt.NewSecretGenerator(10), totp.NewAuthenticator("TestIssuer", 20),
		s.notifierAPI, s.logOperation, 10,
	)

	codes, err := uc.Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token", "000000")
	s.Require().Error(err)
	s.Nil(codes)
	s.Equal(entity.Auth2FA{}, s.saved)
	s.Empty(s.deleted)
	s.False(s.notified)
	// неверный TOTP-код - это неудачное подтверждение, а не блокировка
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.ConfirmFailed, s.logEntries[0].LogStatus)
	s.Equal(logreason.WrongCode, s.logEntries[0].Reason)
}
