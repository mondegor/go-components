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
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
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

	securityLog    *mock.MocksecurityLogStorage
	securityEvents []entity.SecurityLogEvent // записанные в журнал безопасности события
	securityLogErr error                     // ошибка, которую вернёт запись в журнал безопасности

	notified     bool
	notifiedKey  string
	notifiedWith map[string]any
	notifyErr    error // ошибка, которую вернёт отправка уведомления
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
	s.securityLog = mock.NewMocksecurityLogStorage(s.ctrl)
	s.securityEvents = nil
	s.securityLogErr = nil
	s.notified = false
	s.notifiedKey = ""
	s.notifiedWith = nil
	s.notifyErr = nil

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
			if s.notifyErr != nil {
				return s.notifyErr
			}

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

	s.securityLog.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, row entity.SecurityLogEvent) error {
			if s.securityLogErr != nil {
				return s.securityLogErr
			}

			s.securityEvents = append(s.securityEvents, row)

			return nil
		}).
		AnyTimes()
}

type ApplyTOTPSuite struct {
	baseSuite

	binder   *mock.Mockuser2faBinder
	verifier *mock.MockoperationDeleter
	revoker  *mock.MockoperationRevoker
	saved    entity.Auth2FA
	deleted  string
	bindErr  error // ошибка, которую вернёт привязка 2FA (по умолчанию привязка успешна)

	deleteErr error // ошибка, которую вернёт удаление операции

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
	s.deleteErr = nil
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
			if s.deleteErr != nil {
				return s.deleteErr
			}

			s.deleted = token

			return nil
		}).
		AnyTimes()
}

// SetupSubTest - каждый подтест стартует с собственными моками и состоянием набора.
func (s *ApplyTOTPSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ApplyTOTPSuite) TestValidCodeBindsAndReturnsCodes() {
	userID := uuid.New()
	op := confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`)

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)

	auth := totp.NewAuthenticator("TestIssuer", 20)
	uc := security.NewApplyTOTPGenerator(
		s.txManager, s.binder, s.verifier, s.revoker,
		crypt.NewSecretGenerator(), auth, s.notifierAPI, s.actorProps, s.logOperation, s.securityLog, 10, 10,
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
	// включение 2FA записывается в журнал безопасности с типом фактора
	s.Require().Len(s.securityEvents, 1)
	s.Equal(userID, s.securityEvents[0].UserID)
	s.Equal(securityevent.Auth2FAEnabled, s.securityEvents[0].EventType)
	s.Equal(&entity.SecurityLogExtra{Factor: auth2fatype.TOTP.String()}, s.securityEvents[0].Extra)
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
		crypt.NewSecretGenerator(), auth, s.notifierAPI, s.actorProps, s.logOperation, s.securityLog, 10, 10,
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
	s.Empty(s.securityEvents)
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
		s.notifierAPI, s.actorProps, s.logOperation, s.securityLog, 10, 10,
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

// newUseCase - ApplyTOTPGenerator с реальными генератором кодов и TOTP-валидатором.
func (s *ApplyTOTPSuite) newUseCase(recoveryCount int) *security.ApplyTOTPGenerator {
	return security.NewApplyTOTPGenerator(
		s.txManager, s.binder, s.verifier, s.revoker,
		crypt.NewSecretGenerator(), totp.NewAuthenticator("TestIssuer", 20),
		s.notifierAPI, s.actorProps, s.logOperation, s.securityLog, recoveryCount, 10,
	)
}

// validCode - действующий TOTP-код для testTotpSecret.
func (s *ApplyTOTPSuite) validCode() string {
	code, err := totp.NewAuthenticator("TestIssuer", 20).GenerateCode(testTotpSecret, time.Now())
	s.Require().NoError(err)

	return code
}

// TestInvalidInput - некорректный вход отклоняется до обращения к хранилищу.
func (s *ApplyTOTPSuite) TestInvalidInput() {
	tests := []struct {
		name    string
		userID  uuid.UUID
		token   string
		code    string
		wantErr error
	}{
		{name: "nil user", token: "op-token", code: "123456", wantErr: errors.ErrInternalIncorrectInputData},
		{name: "empty code", userID: uuid.New(), token: "op-token", wantErr: errors.ErrInternalIncorrectInputData},
		{name: "empty token", userID: uuid.New(), code: "123456", wantErr: mrauth.ErrOperationInvalid},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// FetchOneForUpdate не вызывается
			codes, err := s.newUseCase(10).Execute(s.ctx, dto.ActorMeta{UserID: tt.userID}, tt.token, tt.code)
			s.Require().ErrorIs(err, tt.wantErr)
			s.Nil(codes)
			s.Empty(s.logEntries)
		})
	}
}

// TestUnknownTokenIsDomainError - отсутствующая операция - это недействительный токен клиента.
func (s *ApplyTOTPSuite) TestUnknownTokenIsDomainError() {
	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(
		secureoperation.SecureOperation{}, errors.ErrEventStorageNoRecordFound,
	)

	codes, err := s.newUseCase(10).Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, "op-token", "123456")
	s.Require().ErrorIs(err, mrauth.ErrOperationInvalid)
	s.Nil(codes)
	s.Empty(s.logEntries)
}

func (s *ApplyTOTPSuite) TestFetchError() {
	errFetch := errors.New("fetch failed")

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(secureoperation.SecureOperation{}, errFetch)

	codes, err := s.newUseCase(10).Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, "op-token", "123456")
	s.Require().ErrorIs(err, errFetch)
	s.Require().NotErrorIs(err, mrauth.ErrOperationInvalid)
	s.Nil(codes)
	s.Empty(s.logEntries)
}

// TestUnsuitableOperationIsBlocked - чужая, неподходящая или неподтверждённая операция
// отклоняется и фиксируется в журнале как блокировка.
func (s *ApplyTOTPSuite) TestUnsuitableOperationIsBlocked() {
	userID := uuid.New()
	payload := `{"email":"u@e","secret":"` + testTotpSecret + `"}`

	otherUserOp := confirmedOp(uuid.New(), payload)

	otherTypeOp := confirmedOp(userID, payload)
	otherTypeOp.Type = operationtype.ChangePassword

	notConfirmedOp := confirmedOp(userID, payload)
	notConfirmedOp.Status = operationstatus.Opened

	tests := []struct {
		name       string
		op         secureoperation.SecureOperation
		wantErr    error
		wantReason logreason.Enum
	}{
		{name: "other user", op: otherUserOp, wantErr: errors.ErrAccessForbidden, wantReason: logreason.AccessForbidden},
		{name: "other type", op: otherTypeOp, wantErr: errors.ErrAccessForbidden, wantReason: logreason.AccessForbidden},
		{name: "not confirmed", op: notConfirmedOp, wantErr: mrauth.ErrOperationIsNotConfirmed, wantReason: logreason.NotConfirmed},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(tt.op, nil)

			codes, err := s.newUseCase(10).Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token", s.validCode())
			s.Require().ErrorIs(err, tt.wantErr)
			s.Nil(codes)
			s.Equal(entity.Auth2FA{}, s.saved)
			s.Require().Len(s.logEntries, 1)
			s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
			s.Equal(tt.wantReason, s.logEntries[0].Reason)
		})
	}
}

func (s *ApplyTOTPSuite) TestBrokenPayload() {
	userID := uuid.New()

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(confirmedOp(userID, `{`), nil)

	codes, err := s.newUseCase(10).Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token", "123456")
	s.Require().Error(err)
	s.Nil(codes)
	s.Equal(entity.Auth2FA{}, s.saved)
	s.Empty(s.logEntries)
}

func (s *ApplyTOTPSuite) TestValidatorError() {
	userID := uuid.New()
	errValidate := errors.New("validate failed")

	validator := mock.NewMocktotpValidator(s.ctrl)
	validator.EXPECT().ValidateCode("123456", testTotpSecret).Return(false, int64(0), errValidate)

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(
		confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`), nil,
	)

	uc := security.NewApplyTOTPGenerator(
		s.txManager, s.binder, s.verifier, s.revoker,
		crypt.NewSecretGenerator(), validator, s.notifierAPI, s.actorProps, s.logOperation, s.securityLog, 10, 10,
	)

	codes, err := uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token", "123456")
	s.Require().ErrorIs(err, errValidate)
	s.Nil(codes)
	s.Equal(entity.Auth2FA{}, s.saved)
	s.Empty(s.logEntries)
}

func (s *ApplyTOTPSuite) TestRecoveryCodesGeneratorError() {
	userID := uuid.New()
	errGenerate := errors.New("generate failed")

	generator := mock.NewMockrecoveryCodesGenerator(s.ctrl)
	generator.EXPECT().GenerateRecoveryCodes(10, 10).Return(nil, nil, errGenerate)

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(
		confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`), nil,
	)

	uc := security.NewApplyTOTPGenerator(
		s.txManager, s.binder, s.verifier, s.revoker,
		generator, totp.NewAuthenticator("TestIssuer", 20), s.notifierAPI, s.actorProps, s.logOperation, s.securityLog, 10, 10,
	)

	codes, err := uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token", s.validCode())
	s.Require().ErrorIs(err, errGenerate)
	s.Nil(codes)
	s.Equal(entity.Auth2FA{}, s.saved)
	s.Empty(s.logEntries)
}

// TestStepError - сбой любого шага после проверки кода откатывает транзакцию целиком:
// коды не возвращаются, уведомление не уходит, в журнал операций ничего не пишется.
func (s *ApplyTOTPSuite) TestStepError() {
	errStep := errors.New("step failed")

	tests := []struct {
		name  string
		setup func()
	}{
		{name: "bind", setup: func() { s.bindErr = errStep }},
		{name: "delete operation", setup: func() { s.deleteErr = errStep }},
		{name: "revoke operations", setup: func() { s.revokeErr = errStep }},
		{name: "security log", setup: func() { s.securityLogErr = errStep }},
		{name: "notify", setup: func() { s.notifyErr = errStep }},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			userID := uuid.New()

			tt.setup()
			s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(
				confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`), nil,
			)

			codes, err := s.newUseCase(10).Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token", s.validCode())
			s.Require().ErrorIs(err, errStep)
			s.Require().NotErrorIs(err, mrauth.ErrAuth2FAMustBeDisabledFirst)
			s.Nil(codes)
			s.False(s.notified)
			s.Empty(s.logEntries)
		})
	}
}

// TestRecoveryCountClamped - число аварийных кодов из конфигурации зажимается в допустимый диапазон.
func (s *ApplyTOTPSuite) TestRecoveryCountClamped() {
	tests := []struct {
		name          string
		recoveryCount int
		wantCount     int
	}{
		{name: "below min", recoveryCount: 0, wantCount: 2},
		{name: "above max", recoveryCount: 1000, wantCount: 32},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			userID := uuid.New()

			s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(
				confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`), nil,
			)

			codes, err := s.newUseCase(tt.recoveryCount).Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token", s.validCode())
			s.Require().NoError(err)
			s.Len(codes, tt.wantCount)
			s.Len(s.saved.RecoveryCodes, tt.wantCount)
		})
	}
}
