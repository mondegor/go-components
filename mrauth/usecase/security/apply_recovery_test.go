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
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

func confirmedRegenerateOp(userID uuid.UUID) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Token:   "op-token",
		Name:    unit.NameConfirmRegenerateRecovery,
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

	s.updater.EXPECT().
		UpdateRecoveryCodes(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, hashed []string) error {
			s.saved = hashed

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

func (s *ApplyRecoverySuite) newUseCase() *security.ApplyRecovery {
	return security.NewApplyRecovery(
		s.txManager, s.updater, s.verifier,
		crypt.NewSecretGenerator(10), s.notifierAPI, s.logOperation, 8,
	)
}

func (s *ApplyRecoverySuite) TestConfirmedReplacesAndReturnsCodes() {
	userID := uuid.New()

	s.verifier.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(confirmedRegenerateOp(userID), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token")
	s.Require().NoError(err)
	s.Require().Len(codes, 8)
	s.Require().Len(s.saved, 8)
	s.NotEqual(codes, s.saved) // хранятся хеши, возвращается plaintext
	s.Equal("op-token", s.deleted)
	s.True(s.notified)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Applied, s.logEntries[0].LogStatus)
	s.Equal(unit.NameConfirmRegenerateRecovery, s.logEntries[0].OperationName)
}

// TestNo2FARowReportsDisabled - строки 2FA нет: её удалили между созданием операции и её
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

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token")
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

func (s *ApplyRecoverySuite) TestWrongOperationNameNoUpdate() {
	userID := uuid.New()

	// операция чужого типа (confirm.change.totp) не должна применяться как перевыпуск
	s.verifier.EXPECT().
		FetchOneForUpdate(gomock.Any(), gomock.Any()).
		Return(confirmedOp(userID, `{"email":"u@e"}`), nil)

	codes, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{VisitorID: userID}, "op-token")
	s.Require().Error(err)
	s.Nil(codes)
	s.Nil(s.saved)
	s.Empty(s.deleted)
	s.False(s.notified)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.AccessForbidden, s.logEntries[0].Reason)
}
