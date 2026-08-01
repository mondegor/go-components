package security_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	coreerrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/util/conv"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
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

	opener         *mock.MockoperationOpener
	factory2FA     *mock.MockUser2FAConfirmActionCreator
	addressFactory *mock.MockfactoryOperationAddress2FA
	secretFactory  *mock.MockfactoryOperationSecret2FA
	opFactory      *mock.Mockuser2faOperationCreator
	emailChecker   *mock.MockuserEmailChecker
	phoneChecker   *mock.MockuserPhoneChecker
	opened         bool
	openedNote     string
}

func TestChangeSecuritySuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ChangeSecuritySuite))
}

func (s *ChangeSecuritySuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.opener = mock.NewMockoperationOpener(s.ctrl)
	s.factory2FA = mock.NewMockUser2FAConfirmActionCreator(s.ctrl)
	s.addressFactory = mock.NewMockfactoryOperationAddress2FA(s.ctrl)
	s.secretFactory = mock.NewMockfactoryOperationSecret2FA(s.ctrl)
	s.opFactory = mock.NewMockuser2faOperationCreator(s.ctrl)
	s.emailChecker = mock.NewMockuserEmailChecker(s.ctrl)
	s.phoneChecker = mock.NewMockuserPhoneChecker(s.ctrl)
	s.opened = false
	s.openedNote = ""
}

// expectOpen - открытие операции проходит успешно либо возвращает ошибку; попутно
// запоминается имя шаблона уведомления, которое usecase передал компоненту.
func (s *ChangeSecuritySuite) expectOpen(err error) {
	s.opener.EXPECT().
		Open(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ dto.ActorMeta, _ secureoperation.SecureOperation, noteName string, _ conv.Group) error {
			if err != nil {
				return err
			}

			s.opened = true
			s.openedNote = noteName

			return nil
		}).
		AnyTimes()
}

func (s *ChangeSecuritySuite) expect2FA(user dto.User2FA, err error) {
	s.factory2FA.EXPECT().CreateByUserID(gomock.Any(), gomock.Any()).Return(user, err).AnyTimes()
	s.factory2FA.EXPECT().CreateByUserLogin(gomock.Any(), gomock.Any()).Return(user, err).AnyTimes()
}

// expectValueFactory - фабрики операций (по адресу и по секрету) отрабатывают одинаково:
// тесты не различают их, поэтому ожидание ставится сразу на обе.
func (s *ChangeSecuritySuite) expectValueFactory(op secureoperation.SecureOperation, err error) {
	s.addressFactory.EXPECT().Create(gomock.Any(), gomock.Any()).Return(op, err).AnyTimes()
	s.secretFactory.EXPECT().Create(gomock.Any(), gomock.Any()).Return(op, err).AnyTimes()
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
	return security.NewChangeEmailProperty(s.opener, s.emailChecker, s.factory2FA, s.addressFactory)
}

func (s *ChangeSecuritySuite) newChangePassword() *security.ChangePasswordProperty {
	return security.NewChangePasswordProperty(s.opener, s.factory2FA, s.secretFactory)
}

func (s *ChangeSecuritySuite) newChangePhone() *security.ChangePhoneProperty {
	return security.NewChangePhoneProperty(s.opener, s.phoneChecker, s.factory2FA, s.addressFactory)
}

func (s *ChangeSecuritySuite) newChangeTOTP() *security.ChangeTOTPGeneratorProperty {
	return security.NewChangeTOTPGeneratorProperty(s.opener, s.factory2FA, s.opFactory)
}

func (s *ChangeSecuritySuite) newDisable2FA() *security.Disable2FA {
	return security.NewDisable2FA(s.opener, s.factory2FA, s.opFactory)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertyNilUserID() {
	s.expectOpen(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(secureoperation.SecureOperation{}, nil)
	s.expectEmailChecker(nil)

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{}, contactaddress.NewEmail("new@example.com"))
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertySuccess() {
	s.expectOpen(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectEmailChecker(nil)

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, contactaddress.NewEmail("new@example.com"))
	s.Require().NoError(err)
	s.True(s.opened)
	s.Equal("confirm.change.email", s.openedNote)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertyEmailUnavailable() {
	s.expectOpen(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectEmailChecker(errors.New("taken"))

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, contactaddress.NewEmail("new@example.com"))
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertyUser2FAFactoryError() {
	s.expectOpen(nil)
	s.expect2FA(dto.User2FA{}, errors.New("no user"))
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectEmailChecker(nil)

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, contactaddress.NewEmail("new@example.com"))
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeEmailPropertyOpenError() {
	s.expectOpen(errors.New("open failed"))
	s.expect2FA(userWithEmail(), nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectEmailChecker(nil)

	_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, contactaddress.NewEmail("new@example.com"))
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangePasswordPropertyNilUserID() {
	s.expectOpen(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(secureoperation.SecureOperation{}, nil)

	_, err := s.newChangePassword().Execute(s.ctx, dto.ActorMeta{}, "new-password")
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangePasswordPropertySuccess() {
	s.expectOpen(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)

	_, err := s.newChangePassword().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "new-password")
	s.Require().NoError(err)
	s.True(s.opened)
	s.Equal("confirm.change.password", s.openedNote)
}

func (s *ChangeSecuritySuite) TestChangePasswordPropertyFactoryError() {
	s.expectOpen(nil)
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
			s.expectOpen(nil)
			s.expect2FA(userWith2FA(method), nil)
			s.expectValueFactory(openedEmailOp(s.T()), nil)

			_, err := s.newChangePassword().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "new-password")
			s.Require().ErrorIs(err, mrauth.ErrAuth2FAMustBeDisabledFirst)
			s.False(s.opened)
		})
	}
}

// новое значение свойства провалидировано на границе ввода,
// поэтому пустое значение здесь - ошибка проводки, а не клиента.
func (s *ChangeSecuritySuite) TestChangePropertyEmptyNewValue() {
	s.expectOpen(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectEmailChecker(nil)
	s.expectPhoneChecker(nil)

	actor := dto.ActorMeta{VisitorID: uuid.New()}

	s.Run("empty newEmail", func() {
		_, err := s.newChangeEmail().Execute(s.ctx, actor, contactaddress.ContactAddress{})
		s.Require().ErrorIs(err, coreerrors.ErrInternalIncorrectInputData)
	})

	s.Run("empty newPhone", func() {
		_, err := s.newChangePhone().Execute(s.ctx, actor, contactaddress.ContactAddress{})
		s.Require().ErrorIs(err, coreerrors.ErrInternalIncorrectInputData)
	})

	s.Run("empty newPassword", func() {
		_, err := s.newChangePassword().Execute(s.ctx, actor, "")
		s.Require().ErrorIs(err, coreerrors.ErrInternalIncorrectInputData)
	})
}

func (s *ChangeSecuritySuite) TestChangePhonePropertyNilUserID() {
	s.expectOpen(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(secureoperation.SecureOperation{}, nil)
	s.expectPhoneChecker(nil)

	_, err := s.newChangePhone().Execute(s.ctx, dto.ActorMeta{}, contactaddress.NewPhone("79991234567"))
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangePhonePropertySuccess() {
	s.expectOpen(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectPhoneChecker(nil)

	_, err := s.newChangePhone().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, contactaddress.NewPhone("79991234567"))
	s.Require().NoError(err)
	s.True(s.opened)
	s.Equal("confirm.change.phone", s.openedNote)
}

func (s *ChangeSecuritySuite) TestChangePhonePropertyPhoneUnavailable() {
	s.expectOpen(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectValueFactory(openedEmailOp(s.T()), nil)
	s.expectPhoneChecker(errors.New("taken"))

	_, err := s.newChangePhone().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, contactaddress.NewPhone("79991234567"))
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeTOTPGeneratorPropertyNilUserID() {
	s.expectOpen(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectOpFactory(secureoperation.SecureOperation{}, nil)

	_, err := s.newChangeTOTP().Execute(s.ctx, dto.ActorMeta{})
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeTOTPGeneratorPropertySuccess() {
	s.expectOpen(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectOpFactory(openedEmailOp(s.T()), nil)

	_, err := s.newChangeTOTP().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
	s.Require().NoError(err)
	s.True(s.opened)
	s.Equal("confirm.change.totp", s.openedNote)
}

func (s *ChangeSecuritySuite) TestChangeTOTPGeneratorPropertyFactoryError() {
	s.expectOpen(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectOpFactory(secureoperation.SecureOperation{}, errors.New("factory failed"))

	_, err := s.newChangeTOTP().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestChangeTOTPGeneratorPropertyOpenError() {
	s.expectOpen(errors.New("open failed"))
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
			s.expectOpen(nil)
			s.expect2FA(userWith2FA(method), nil)
			s.expectOpFactory(openedEmailOp(s.T()), nil)

			_, err := s.newChangeTOTP().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
			s.Require().ErrorIs(err, mrauth.ErrAuth2FAMustBeDisabledFirst)
			s.False(s.opened)
		})
	}
}

func (s *ChangeSecuritySuite) TestDisable2FANilUserID() {
	s.expectOpen(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectOpFactory(secureoperation.SecureOperation{}, nil)

	_, err := s.newDisable2FA().Execute(s.ctx, dto.ActorMeta{})
	s.Require().Error(err)
}

func (s *ChangeSecuritySuite) TestDisable2FASuccess() {
	s.expectOpen(nil)
	s.expect2FA(userWithEmail(), nil)
	s.expectOpFactory(openedEmailOp(s.T()), nil)

	_, err := s.newDisable2FA().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
	s.Require().NoError(err)
	s.True(s.opened)
	s.Equal("confirm.disable.2fa", s.openedNote)
}

func (s *ChangeSecuritySuite) TestDisable2FAFactoryError() {
	s.expectOpen(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectOpFactory(secureoperation.SecureOperation{}, errors.New("factory failed"))

	_, err := s.newDisable2FA().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})
	s.Require().Error(err)
}

// TestUserRowIsMissingIsInternal - строки пользователя нет, хотя access-токен предъявлен и
// валиден: это рассогласованное состояние БД, а не ответ клиенту. Наружу должна идти внутренняя
// ошибка (500), а не ошибка о недействительном токене и не errors.ErrRecordNotFound (404).
// Проверяются все создающие методы: обёртка ошибок задаётся в каждом конструкторе отдельно,
// поэтому одного usecase здесь недостаточно.
func (s *ChangeSecuritySuite) TestUserRowIsMissingIsInternal() {
	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "change email",
			call: func() error {
				_, err := s.newChangeEmail().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, contactaddress.NewEmail("new@example.com"))

				return err
			},
		},
		{
			name: "change phone",
			call: func() error {
				_, err := s.newChangePhone().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, contactaddress.NewPhone("79991234567"))

				return err
			},
		},
		{
			name: "change password",
			call: func() error {
				_, err := s.newChangePassword().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "new-password")

				return err
			},
		},
		{
			name: "change totp",
			call: func() error {
				_, err := s.newChangeTOTP().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})

				return err
			},
		},
		{
			name: "disable 2fa",
			call: func() error {
				_, err := s.newDisable2FA().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})

				return err
			},
		},
		{
			name: "regenerate recovery codes",
			call: func() error {
				_, err := s.newRegenerateRecovery().Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()})

				return err
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.expectOpen(nil)
			s.expectEmailChecker(nil)
			s.expectPhoneChecker(nil)
			s.expectValueFactory(secureoperation.SecureOperation{}, nil)
			s.expectOpFactory(secureoperation.SecureOperation{}, nil)

			// фабрика второго фактора не нашла пользователя
			s.expect2FA(dto.User2FA{}, coreerrors.ErrEventStorageNoRecordFound)

			err := tt.call()
			s.Require().Error(err)
			s.Require().NotErrorIs(err, coreerrors.ErrRecordNotFound)
			s.Require().NotErrorIs(err, secureoperation.ErrOperationInvalid)
		})
	}
}

func (s *ChangeSecuritySuite) newRegenerateRecovery() *security.RegenerateRecoveryProperty {
	return security.NewRegenerateRecoveryProperty(s.opener, s.factory2FA, s.opFactory)
}
