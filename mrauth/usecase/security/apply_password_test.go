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
	s.revokedFor = uuid.Nil
	s.revokeReason = logreason.Unspecified
	s.revokeErr = nil

	s.revoker.EXPECT().
		RevokeAll(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, actor dto.ActorMeta, reason logreason.Enum) error {
			s.revokedFor = actor.VisitorID
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

func (s *ApplyPasswordSuite) newUseCase() *security.ApplyPassword {
	return security.NewApplyPassword(
		s.txManager, s.binder, s.verifier, s.revoker,
		crypt.NewSecretGenerator(), s.notifierAPI, s.logOperation, 8, 10,
	)
}

func (s *ApplyPasswordSuite) TestConfirmedBindsAndReturnsCodes() {
	userID := uuid.New()

	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedPasswordOp(userID, `{"new_password":"hashed-pwd","email":"u@e"}`), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token")
	s.Require().NoError(err)
	s.Require().Len(codes, 8)
	s.Equal(auth2fatype.Password, s.saved.Type)
	s.Equal("hashed-pwd", s.saved.Secret) // секрет уже захеширован при создании операции
	s.Require().Len(s.saved.RecoveryCodes, 8)
	s.NotEqual(codes, s.saved.RecoveryCodes) // хранятся хеши, возвращается plaintext
	s.Equal("op-token", s.deleted)
	s.True(s.notified)
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

	first, err := uc.Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token")
	s.Require().NoError(err)

	second, err := uc.Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token")
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

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token")
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

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token")
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

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token")
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

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token")
	s.Require().Error(err)
	s.Nil(codes)
	s.False(s.notified)
}
