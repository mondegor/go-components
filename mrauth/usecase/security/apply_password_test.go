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
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

func confirmedPasswordOp(userID uuid.UUID, payload string) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Token:   "op-token",
		Type:    operationtype.ChangePassword,
		UserID:  userID,
		Payload: []byte(payload),
		Status:  operationstatus.Confirmed,
	}
}

type ApplyPasswordSuite struct {
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

func TestApplyPasswordSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApplyPasswordSuite))
}

func (s *ApplyPasswordSuite) SetupTest() {
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
func (s *ApplyPasswordSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ApplyPasswordSuite) newUseCase() *security.ApplyPassword {
	return security.NewApplyPassword(
		s.txManager, s.binder, s.verifier, s.revoker,
		crypt.NewSecretGenerator(), s.notifierAPI, s.actorProps, s.logOperation, s.securityLog, 8, 10,
	)
}

func (s *ApplyPasswordSuite) TestConfirmedBindsAndReturnsCodes() {
	userID := uuid.New()

	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedPasswordOp(userID, `{"new_password":"hashed-pwd","email":"u@e"}`), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().NoError(err)
	s.Require().Len(codes, 8)
	s.Equal(auth2fatype.Password, s.saved.Type)
	s.Equal("hashed-pwd", s.saved.Secret) // секрет уже захеширован при создании операции
	s.Require().Len(s.saved.RecoveryCodes, 8)
	s.NotEqual(codes, s.saved.RecoveryCodes) // хранятся хеши, возвращается plaintext
	s.Equal("op-token", s.deleted)
	s.True(s.notified)
	// о включении 2FA уведомляет одно событие для любого фактора, фактор - в props
	s.Equal("user.2fa.enabled", s.notifiedKey)
	s.Equal("u@e", s.notifiedWith["to"])
	s.Equal(auth2fatype.Password.String(), s.notifiedWith["factor"])
	s.Equal("TestApp, TestDevice", s.notifiedWith["device"])
	s.Contains(s.notifiedWith, "occurredAt")
	s.Contains(s.notifiedWith, "ip")
	// включение 2FA записывается в журнал безопасности с типом фактора
	s.Require().Len(s.securityEvents, 1)
	s.Equal(userID, s.securityEvents[0].UserID)
	s.Equal(securityevent.Auth2FAEnabled, s.securityEvents[0].EventType)
	s.Equal(&entity.SecurityLogExtra{Factor: auth2fatype.Password.String()}, s.securityEvents[0].Extra)
	// включение 2FA отзывает все незавершённые операции пользователя
	s.Equal(userID, s.revokedFor)
	s.Equal(logreason.Auth2FAStateChanged, s.revokeReason)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Applied, s.logEntries[0].LogStatus)
	s.Equal(operationtype.ChangePassword.String(), s.logEntries[0].SourceName)
}

func (s *ApplyPasswordSuite) TestReissuesNewCodesEachTime() {
	userID := uuid.New()
	payload := `{"new_password":"hashed-pwd","email":"u@e"}`

	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedPasswordOp(userID, payload), nil).
		Times(2)

	uc := s.newUseCase()

	first, err := uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().NoError(err)

	second, err := uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().NoError(err)

	s.NotEqual(first, second) // каждая смена пароля выдаёт новый набор кодов
}

// TestPayloadWithoutPasswordNoBind - payload без пароля отклоняется разбором на чтении:
// пароль не привязывается, коды не выдаются.
func (s *ApplyPasswordSuite) TestPayloadWithoutPasswordNoBind() {
	userID := uuid.New()

	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedPasswordOp(userID, `{"email":"u@e"}`), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().Error(err)
	s.Nil(codes)
	s.Empty(s.saved.Secret, "пароль не должен привязываться")
}

// TestActive2FAConflictNoApply - 2FA включили другим способом между созданием операции
// и её применением: привязка отклоняется нарушением уникальности, операция остаётся
// неприменённой, а наружу уходит ErrAuth2FAMustBeDisabledFirst (409).
func (s *ApplyPasswordSuite) TestActive2FAConflictNoApply() {
	userID := uuid.New()
	s.bindErr = errors.ErrInternalStorageDuplicateKeyViolation.New()

	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedPasswordOp(userID, `{"new_password":"hashed-pwd","email":"u@e"}`), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
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

func (s *ApplyPasswordSuite) TestWrongOperationTypeNoBind() {
	userID := uuid.New()

	// операция чужого типа (confirm.change.totp) не должна применяться как смена пароля
	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedOp(userID, `{"new_password":"hashed-pwd","email":"u@e"}`), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().Error(err)
	s.Nil(codes)
	s.Empty(s.deleted)
	s.False(s.notified)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.AccessForbidden, s.logEntries[0].Reason)
}

// TestRevokeError - отзыв операций входит в применение: его ошибка отменяет применение
// целиком (транзакция откатывается, уведомление не отправляется), иначе 2FA включилась бы
// при живых операциях, построенных без второго фактора.
func (s *ApplyPasswordSuite) TestRevokeError() {
	userID := uuid.New()
	s.revokeErr = errors.ErrInternalStorageQueryFailed.New()

	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedPasswordOp(userID, `{"new_password":"hashed-pwd","email":"u@e"}`), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().Error(err)
	s.Nil(codes)
	s.False(s.notified)
}

// TestSecurityLogError - запись в журнал безопасности входит в применение: её ошибка отменяет
// применение целиком (транзакция откатывается, уведомление не отправляется, коды не выдаются).
func (s *ApplyPasswordSuite) TestSecurityLogError() {
	userID := uuid.New()
	s.securityLogErr = errors.ErrInternalStorageQueryFailed.New()

	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedPasswordOp(userID, `{"new_password":"hashed-pwd","email":"u@e"}`), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().ErrorIs(err, errors.ErrInternalStorageQueryFailed)
	s.Nil(codes)
	s.False(s.notified)
}

// TestInvalidInput - некорректный вход отклоняется до обращения к хранилищу.
func (s *ApplyPasswordSuite) TestInvalidInput() {
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
func (s *ApplyPasswordSuite) TestUnknownTokenIsDomainError() {
	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(
		secureoperation.SecureOperation{}, errors.ErrEventStorageNoRecordFound,
	)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, "op-token")
	s.Require().ErrorIs(err, mrauth.ErrOperationInvalid)
	s.Nil(codes)
	s.Empty(s.logEntries)
}

func (s *ApplyPasswordSuite) TestFetchError() {
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
func (s *ApplyPasswordSuite) TestUnsuitableOperationIsBlocked() {
	userID := uuid.New()
	payload := `{"new_password":"hashed-pwd","email":"u@e"}`

	notConfirmedOp := confirmedPasswordOp(userID, payload)
	notConfirmedOp.Status = operationstatus.Opened

	tests := []struct {
		name       string
		op         secureoperation.SecureOperation
		wantErr    error
		wantReason logreason.Enum
	}{
		{name: "other user", op: confirmedPasswordOp(uuid.New(), payload), wantErr: errors.ErrAccessForbidden, wantReason: logreason.AccessForbidden},
		{name: "not confirmed", op: notConfirmedOp, wantErr: mrauth.ErrOperationIsNotConfirmed, wantReason: logreason.NotConfirmed},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(tt.op, nil)

			codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
			s.Require().ErrorIs(err, tt.wantErr)
			s.Nil(codes)
			s.Equal(entity.Auth2FA{}, s.saved)
			s.Require().Len(s.logEntries, 1)
			s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
			s.Equal(tt.wantReason, s.logEntries[0].Reason)
		})
	}
}

func (s *ApplyPasswordSuite) TestRecoveryCodesGeneratorError() {
	userID := uuid.New()
	errGenerate := errors.New("generate failed")

	generator := mock.NewMockrecoveryCodesGenerator(s.ctrl)
	generator.EXPECT().GenerateRecoveryCodes(8, 10).Return(nil, nil, errGenerate)

	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), "op-token").
		Return(confirmedPasswordOp(userID, `{"new_password":"hashed-pwd","email":"u@e"}`), nil)

	uc := security.NewApplyPassword(
		s.txManager, s.binder, s.verifier, s.revoker,
		generator, s.notifierAPI, s.actorProps, s.logOperation, s.securityLog, 8, 10,
	)

	codes, err := uc.Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
	s.Require().ErrorIs(err, errGenerate)
	s.Nil(codes)
	s.Equal(entity.Auth2FA{}, s.saved)
	s.Empty(s.logEntries)
}

// TestStepError - сбой привязки (кроме конфликта с активной 2FA), удаления операции или
// отправки уведомления откатывает применение целиком: коды не возвращаются, в журнал
// операций ничего не пишется.
func (s *ApplyPasswordSuite) TestStepError() {
	errStep := errors.New("step failed")

	tests := []struct {
		name  string
		setup func()
	}{
		{name: "bind", setup: func() { s.bindErr = errStep }},
		{name: "delete operation", setup: func() { s.deleteErr = errStep }},
		{name: "notify", setup: func() { s.notifyErr = errStep }},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			userID := uuid.New()

			tt.setup()
			s.verifier.EXPECT().
				FetchOneForUpdate(gomock.Any(), "op-token").
				Return(confirmedPasswordOp(userID, `{"new_password":"hashed-pwd","email":"u@e"}`), nil)

			codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{UserID: userID}, "op-token")
			s.Require().ErrorIs(err, errStep)
			s.Require().NotErrorIs(err, mrauth.ErrAuth2FAMustBeDisabledFirst)
			s.Nil(codes)
			s.False(s.notified)
			s.Empty(s.logEntries)
		})
	}
}
