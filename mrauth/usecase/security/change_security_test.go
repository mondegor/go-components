package security_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

// openedEmailOp - sendable-операция Email, при Notify отправляющая код через notifier.
func openedEmailOp(t *testing.T) secureoperation.SecureOperation {
	t.Helper()

	op, err := secureoperation.NewOperation(
		"op-token",
		"confirm.change",
		uuid.New(),
		[]secureoperation.ConfirmAction{
			{
				Method:           confirmmethod.Email,
				MaxAttempts:      3,
				MaxResends:       5,
				MinResendTime:    5 * time.Minute,
				Expiry:           10 * time.Minute,
				Address:          "u@e",
				ConfirmCode:      "code123", // в хранилище идёт хеш
				PlainConfirmCode: "code123", // открытый код - для отправки через Notify
			},
		},
		nil,
	)
	require.NoError(t, err)

	return op
}

func userWithEmail() dto.User2FA {
	return dto.User2FA{ID: uuid.New(), Email: "user@example.com"}
}

// userWith2FA - пользователь с уже включённым вторым фактором указанного типа.
func userWith2FA(method confirmmethod.Enum) dto.User2FA {
	return dto.User2FA{
		ID:        uuid.New(),
		Email:     "user@example.com",
		Action2FA: secureoperation.ConfirmAction{Method: method},
	}
}

type ChangeSecuritySuite struct {
	baseSuite

	creator      *mock.MockoperationCreator
	factory2FA   *mock.MockUser2FAConfirmActionCreator
	valueFactory *mock.MockfactoryOperationValue2FA
	opFactory    *mock.Mockuser2faOperationCreator
	emailChecker *mock.MockuserEmailChecker
	phoneChecker *mock.MockuserPhoneChecker
	inserted     bool
}

func TestChangeSecuritySuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ChangeSecuritySuite))
}

func (s *ChangeSecuritySuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.creator = mock.NewMockoperationCreator(s.ctrl)
	s.factory2FA = mock.NewMockUser2FAConfirmActionCreator(s.ctrl)
	s.valueFactory = mock.NewMockfactoryOperationValue2FA(s.ctrl)
	s.opFactory = mock.NewMockuser2faOperationCreator(s.ctrl)
	s.emailChecker = mock.NewMockuserEmailChecker(s.ctrl)
	s.phoneChecker = mock.NewMockuserPhoneChecker(s.ctrl)
	s.inserted = false
}

// expectInsert - хранилище операций принимает вставку либо возвращает ошибку.
func (s *ChangeSecuritySuite) expectInsert(err error) {
	s.creator.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, secureoperation.SecureOperation) error {
			if err != nil {
				return err
			}

			s.inserted = true

			return nil
		}).
		AnyTimes()
}

func (s *ChangeSecuritySuite) expect2FA(user dto.User2FA, err error) {
	s.factory2FA.EXPECT().CreateByUserID(gomock.Any(), gomock.Any()).Return(user, err).AnyTimes()
	s.factory2FA.EXPECT().CreateByUserLogin(gomock.Any(), gomock.Any()).Return(user, err).AnyTimes()
}

func (s *ChangeSecuritySuite) expectValueFactory(op secureoperation.SecureOperation, err error) {
	s.valueFactory.EXPECT().Create(gomock.Any(), gomock.Any()).Return(op, err).AnyTimes()
}

func (s *ChangeSecuritySuite) expectOpFactory(op secureoperation.SecureOperation, err error) {
	s.opFactory.EXPECT().Create(gomock.Any()).Return(op, err).AnyTimes()
}

func (s *ChangeSecuritySuite) expectEmailChecker(err error) {
	s.emailChecker.EXPECT().CheckAvailabilityEmail(gomock.Any(), gomock.Any()).Return(err).AnyTimes()
}

func (s *ChangeSecuritySuite) expectPhoneChecker(err error) {
	s.phoneChecker.EXPECT().CheckAvailabilityPhone(gomock.Any(), gomock.Any()).Return(err).AnyTimes()
}

func (s *ChangeSecuritySuite) newChangeEmail() *security.ChangeEmailProperty {
	return security.NewChangeEmailProperty(
		s.txManager, s.creator, s.emailChecker, s.notifierAPI,
		s.factory2FA, s.valueFactory, s.logOperation,
	)
}

func (s *ChangeSecuritySuite) newChangePassword() *security.ChangePasswordProperty {
	return security.NewChangePasswordProperty(
		s.txManager, s.creator, s.notifierAPI,
		s.factory2FA, s.valueFactory, s.logOperation,
	)
}

func (s *ChangeSecuritySuite) newChangePhone() *security.ChangePhoneProperty {
	return security.NewChangePhoneProperty(
		s.txManager, s.creator, s.phoneChecker, s.notifierAPI,
		s.factory2FA, s.valueFactory, s.logOperation,
	)
}

func (s *ChangeSecuritySuite) newChangeTOTP() *security.ChangeTOTPGeneratorProperty {
	return security.NewChangeTOTPGeneratorProperty(
		s.txManager, s.creator, s.notifierAPI,
		s.factory2FA, s.opFactory, s.logOperation,
	)
}

func (s *ChangeSecuritySuite) newDisable2FA() *security.Disable2FA {
	return security.NewDisable2FA(
		s.txManager, s.creator, s.notifierAPI,
		s.factory2FA, s.opFactory, s.logOperation,
	)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertyNilUserID() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(secureoperation.SecureOperation{}, nil)
	s.expectEmailChecker(nil)

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{}, "new@example.com")
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertySuccess() {
	s.expectInsert(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectEmailChecker(nil)

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "new@example.com")
	s.Require().NoError(err)
	s.True(s.inserted)
	s.True(s.notified)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertyInvalidEmail() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(secureoperation.SecureOperation{}, nil)
	s.expectEmailChecker(nil)

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "bad")
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertyEmailUnavailable() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectEmailChecker(errors.New("taken"))

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "new@example.com")
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertyUser2FAFactoryError() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, errors.New("no user"))
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectEmailChecker(nil)

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "new@example.com")
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertyInsertError() {
	s.expectInsert(errors.New("insert failed"))
	s.expect2FA(userWithEmail(), nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectEmailChecker(nil)

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "new@example.com")
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangePasswordPropertyNilUserID() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(secureoperation.SecureOperation{}, nil)

	_, err := s.newChangePassword().Execute(s.ctx, dto.ActorMeta{}, "new-password")
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangePasswordPropertySuccess() {
	s.expectInsert(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)

	_, err := s.newChangePassword().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "new-password")
	s.Require().NoError(err)
	s.True(s.inserted)
	s.True(s.notified)
}

func (s *ChangeSecuritySuite) TestChangePasswordPropertyFactoryError() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(secureoperation.SecureOperation{}, errors.New("factory failed"))

	_, err := s.newChangePassword().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "new-password")
	s.Require().Error(err)
}

// TestChangePasswordPropertyRejectedWhen2FAActive - активный 2FA любого типа нужно сначала
// отключить (disable), нельзя менять на месте.
func (s *ChangeSecuritySuite) TestChangePasswordPropertyRejectedWhen2FAActive() {
	for _, method := range []confirmmethod.Enum{confirmmethod.Password, confirmmethod.TOTP} {
		s.Run(method.String(), func() {
			s.expectInsert(nil)
			s.expect2FA(userWith2FA(method), nil)
			s.expectValueFactory(openedEmailOp(s.T()), nil)

			_, err := s.newChangePassword().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "new-password")
			s.Require().ErrorIs(err, mrauth.Err2FAMustBeDisabledFirst)
			s.False(s.inserted)
		})
	}
}

func (s *ChangeSecuritySuite) TestChangePhonePropertyNilUserID() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(secureoperation.SecureOperation{}, nil)
	s.expectPhoneChecker(nil)

	_, err := s.newChangePhone().Execute(s.ctx, dto.ActorMeta{}, "79991234567")
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangePhonePropertySuccess() {
	s.expectInsert(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectPhoneChecker(nil)

	_, err := s.newChangePhone().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "79991234567")
	s.Require().NoError(err)
	s.True(s.inserted)
	s.True(s.notified)
}

func (s *ChangeSecuritySuite) TestChangePhonePropertyInvalidPhone() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(secureoperation.SecureOperation{}, nil)
	s.expectPhoneChecker(nil)

	_, err := s.newChangePhone().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "bad")
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangePhonePropertyPhoneUnavailable() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectPhoneChecker(errors.New("taken"))

	_, err := s.newChangePhone().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "79991234567")
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeTOTPGeneratorPropertyNilUserID() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectOpFactory(secureoperation.SecureOperation{}, nil)

	_, err := s.newChangeTOTP().Execute(s.ctx, dto.ActorMeta{})
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeTOTPGeneratorPropertySuccess() {
	s.expectInsert(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectOpFactory(openedEmailOp(s.T()), nil)

	_, err := s.newChangeTOTP().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
	s.Require().NoError(err)
	s.True(s.inserted)
	s.True(s.notified)
}

func (s *ChangeSecuritySuite) TestChangeTOTPGeneratorPropertyFactoryError() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectOpFactory(secureoperation.SecureOperation{}, errors.New("factory failed"))

	_, err := s.newChangeTOTP().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeTOTPGeneratorPropertyInsertError() {
	s.expectInsert(errors.New("insert failed"))
	s.expect2FA(userWithEmail(), nil)
	s.expectOpFactory(openedEmailOp(s.T()), nil)

	_, err := s.newChangeTOTP().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
	s.Require().Error(err)
}

// TestChangeTOTPGeneratorPropertyRejectedWhen2FAActive - активный 2FA любого типа нужно сначала
// отключить (disable), нельзя менять на месте.
func (s *ChangeSecuritySuite) TestChangeTOTPGeneratorPropertyRejectedWhen2FAActive() {
	for _, method := range []confirmmethod.Enum{confirmmethod.Password, confirmmethod.TOTP} {
		s.Run(method.String(), func() {
			s.expectInsert(nil)
			s.expect2FA(userWith2FA(method), nil)
			s.expectOpFactory(openedEmailOp(s.T()), nil)

			_, err := s.newChangeTOTP().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
			s.Require().ErrorIs(err, mrauth.Err2FAMustBeDisabledFirst)
			s.False(s.inserted)
		})
	}
}

func (s *ChangeSecuritySuite) TestDisable2FANilUserID() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectOpFactory(secureoperation.SecureOperation{}, nil)

	_, err := s.newDisable2FA().Execute(s.ctx, dto.ActorMeta{})
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestDisable2FASuccess() {
	s.expectInsert(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectOpFactory(openedEmailOp(s.T()), nil)

	_, err := s.newDisable2FA().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
	s.Require().NoError(err)
	s.True(s.inserted)
	s.True(s.notified)
}

func (s *ChangeSecuritySuite) TestDisable2FAFactoryError() {
	s.expectInsert(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectOpFactory(secureoperation.SecureOperation{}, errors.New("factory failed"))

	_, err := s.newDisable2FA().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
	s.Require().Error(err)
}
