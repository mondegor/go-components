package security_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

//go:generate mockgen -source=apply_recovery.go -destination=mock/apply_recovery.go -package=mock

func confirmedRegenerateOp(userID uuid.UUID) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Token:   "op-token",
		Type:    operationtype.RegenerateRecovery,
		UserID:  userID,
		Payload: []byte(`{"email":"u@e"}`),
		Status:  operationstatus.Confirmed,
	}
}

type ApplyRecoverySuite struct {
	baseSuite

	updater  *mock.MockrecoveryCodesUpdater
	verifier *mock.MockoperationDeleter
	saved    []string
	deleted  string

	updateErr error // ошибка, которую вернёт замена аварийных кодов
	deleteErr error // ошибка, которую вернёт удаление операции
}

func TestApplyRecoverySuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApplyRecoverySuite))
}

func (s *ApplyRecoverySuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.updater = mock.NewMockrecoveryCodesUpdater(s.ctrl)
	s.verifier = mock.NewMockoperationDeleter(s.ctrl)
	s.saved = nil
	s.deleted = ""
	s.updateErr = nil
	s.deleteErr = nil

	s.updater.EXPECT().
		UpdateRecoveryCodes(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, hashed []string) error {
			if s.updateErr != nil {
				return s.updateErr
			}

			s.saved = hashed

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
func (s *ApplyRecoverySuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ApplyRecoverySuite) newUseCase() *security.ApplyRecovery {
	return security.NewApplyRecovery(
		s.txManager, s.updater, s.verifier,
		crypt.NewSecretGenerator(), s.notifierAPI, s.actorProps, s.logOperation, s.securityLog, 8, 10,
	)
}

func (s *ApplyRecoverySuite) TestConfirmedReplacesAndReturnsCodes() {
	userID := uuid.New()

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(confirmedRegenerateOp(userID), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().NoError(err)
	s.Require().Len(codes, 8)
	s.Require().Len(s.saved, 8)
	s.NotEqual(codes, s.saved) // хранятся хеши, возвращается plaintext
	s.Equal("op-token", s.deleted)
	s.True(s.notified)
	s.Equal("user.recovery_codes.changed", s.notifiedKey)
	s.Equal("TestApp, TestDevice", s.notifiedWith["device"])
	s.Contains(s.notifiedWith, "occurredAt")
	s.Require().Len(s.securityEvents, 1)
	s.Equal(userID, s.securityEvents[0].UserID)
	s.Equal(securityevent.RecoveryCodesRegenerated, s.securityEvents[0].EventType)
	s.Nil(s.securityEvents[0].Extra)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Applied, s.logEntries[0].LogStatus)
	s.Equal(operationtype.RegenerateRecovery.String(), s.logEntries[0].SourceName)
}

// TestNo2FARowReportsDisabled - записи 2FA нет: её удалили между созданием операции и её
// применением. Клиент должен увидеть "2FA выключена", а не ошибку о недействительном токене:
// токен цел, и создавать операцию заново бессмысленно - сначала нужно включить 2FA.
func (s *ApplyRecoverySuite) TestNo2FARowReportsDisabled() {
	userID := uuid.New()

	// заготовка из SetupTest заменяется: здесь обновление обязано сообщить "запись не найдена"
	s.updater = mock.NewMockrecoveryCodesUpdater(s.ctrl)
	s.updater.EXPECT().
		UpdateRecoveryCodes(gomock.Any(), userID, gomock.Any()).
		Return(errors.ErrEventStorageNoRecordFound)

	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedRegenerateOp(userID), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().ErrorIs(err, mrauth.ErrAuth2FAIsDisabled)
	s.Require().NotErrorIs(err, errors.ErrRecordNotFound)
	s.Nil(codes)
	s.Empty(s.deleted)
	s.False(s.notified)
	// гонка с отключением 2FA фиксируется в журнале как блокировка
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.Auth2FAStateChanged, s.logEntries[0].Reason)
}

func (s *ApplyRecoverySuite) TestWrongOperationTypeNoUpdate() {
	userID := uuid.New()

	// операция чужого типа (confirm.change.totp) не должна применяться как перевыпуск
	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedOp(userID, `{"email":"u@e"}`), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().Error(err)
	s.Nil(codes)
	s.Nil(s.saved)
	s.Empty(s.deleted)
	s.False(s.notified)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.AccessForbidden, s.logEntries[0].Reason)
}

// TestInvalidInput - некорректный вход отклоняется до обращения к хранилищу.
func (s *ApplyRecoverySuite) TestInvalidInput() {
	tests := []struct {
		name    string
		userID  uuid.UUID
		token   string
		wantErr error
	}{
		{name: "nil user", token: "op-token", wantErr: errors.ErrInternalIncorrectInputData},
		{name: "empty token", userID: uuid.New(), wantErr: mrauth.ErrOperationInvalid},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// FetchOneForUpdate не вызывается
			codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: tt.userID}, tt.token)
			s.Require().ErrorIs(err, tt.wantErr)
			s.Nil(codes)
			s.Empty(s.logEntries)
		})
	}
}

// TestUnknownTokenIsDomainError - отсутствующая операция - это недействительный токен клиента.
func (s *ApplyRecoverySuite) TestUnknownTokenIsDomainError() {
	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(
		secureoperation.SecureOperation{}, errors.ErrEventStorageNoRecordFound,
	)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, "op-token")
	s.Require().ErrorIs(err, mrauth.ErrOperationInvalid)
	s.Nil(codes)
	s.Empty(s.logEntries)
}

func (s *ApplyRecoverySuite) TestFetchError() {
	errFetch := errors.New("fetch failed")

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(secureoperation.SecureOperation{}, errFetch)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, "op-token")
	s.Require().ErrorIs(err, errFetch)
	s.Require().NotErrorIs(err, mrauth.ErrOperationInvalid)
	s.Nil(codes)
	s.Empty(s.logEntries)
}

// TestUnsuitableOperationIsBlocked - чужая или неподтверждённая операция отклоняется
// и фиксируется в журнале как блокировка.
func (s *ApplyRecoverySuite) TestUnsuitableOperationIsBlocked() {
	userID := uuid.New()

	notConfirmedOp := confirmedRegenerateOp(userID)
	notConfirmedOp.Status = operationstatus.Opened

	tests := []struct {
		name       string
		op         secureoperation.SecureOperation
		wantErr    error
		wantReason logreason.Enum
	}{
		{name: "other user", op: confirmedRegenerateOp(uuid.New()), wantErr: errors.ErrAccessForbidden, wantReason: logreason.AccessForbidden},
		{name: "not confirmed", op: notConfirmedOp, wantErr: mrauth.ErrOperationIsNotConfirmed, wantReason: logreason.NotConfirmed},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(tt.op, nil)

			codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
			s.Require().ErrorIs(err, tt.wantErr)
			s.Nil(codes)
			s.Nil(s.saved)
			s.Require().Len(s.logEntries, 1)
			s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
			s.Equal(tt.wantReason, s.logEntries[0].Reason)
		})
	}
}

func (s *ApplyRecoverySuite) TestBrokenPayload() {
	userID := uuid.New()

	op := confirmedRegenerateOp(userID)
	op.Payload = []byte(`{`)

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(op, nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().Error(err)
	s.Nil(codes)
	s.Nil(s.saved)
	s.Empty(s.logEntries)
}

func (s *ApplyRecoverySuite) TestRecoveryCodesGeneratorError() {
	userID := uuid.New()
	errGenerate := errors.New("generate failed")

	generator := mock.NewMockrecoveryCodesGenerator(s.ctrl)
	generator.EXPECT().GenerateRecoveryCodes(8, 10).Return(nil, nil, errGenerate)

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(confirmedRegenerateOp(userID), nil)

	uc := security.NewApplyRecovery(
		s.txManager, s.updater, s.verifier,
		generator, s.notifierAPI, s.actorProps, s.logOperation, s.securityLog, 8, 10,
	)

	codes, err := uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().ErrorIs(err, errGenerate)
	s.Nil(codes)
	s.Nil(s.saved)
	s.Empty(s.logEntries)
}

// TestStepError - сбой любого шага замены кодов откатывает транзакцию целиком:
// коды не возвращаются, уведомление не уходит, в журнал операций ничего не пишется.
func (s *ApplyRecoverySuite) TestStepError() {
	errStep := errors.New("step failed")

	tests := []struct {
		name  string
		setup func()
	}{
		{name: "update codes", setup: func() { s.updateErr = errStep }},
		{name: "delete operation", setup: func() { s.deleteErr = errStep }},
		{name: "security log", setup: func() { s.securityLogErr = errStep }},
		{name: "notify", setup: func() { s.notifyErr = errStep }},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			userID := uuid.New()

			tt.setup()
			s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(confirmedRegenerateOp(userID), nil)

			codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
			s.Require().ErrorIs(err, errStep)
			s.Require().NotErrorIs(err, mrauth.ErrAuth2FAIsDisabled)
			s.Nil(codes)
			s.False(s.notified)
			s.Empty(s.logEntries)
		})
	}
}
