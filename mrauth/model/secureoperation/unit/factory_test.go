package unit_test

import (
	"encoding/json"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/mock"
)

//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth TokenGenerator,CodeGenerator
//go:generate mockgen -source=change_totp.go -destination=mock/change_totp.go -package=mock

type FactorySuite struct {
	suite.Suite

	ctrl      *gomock.Controller
	tokenGen  *mock.MockTokenGenerator
	codeGen   *mock.MockCodeGenerator
	secretGen *mock.MocktotpSecretGenerator
}

func TestFactorySuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(FactorySuite))
}

func (s *FactorySuite) SetupSubTest() {
	s.SetupTest()
}

func (s *FactorySuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.tokenGen = mock.NewMockTokenGenerator(s.ctrl)
	s.codeGen = mock.NewMockCodeGenerator(s.ctrl)
	s.secretGen = mock.NewMocktotpSecretGenerator(s.ctrl)
}

// expectGenerators - разрешает фабрике сколько угодно раз получать токен операции и код
// подтверждения: конкретное их число зависит от набора действий и здесь не проверяется.
func (s *FactorySuite) expectGenerators() {
	s.tokenGen.EXPECT().GenToken().Return("tok", nil).AnyTimes()
	s.codeGen.EXPECT().GenCodeWithHash().Return("123456", "hashed-code", nil).AnyTimes()
}

// userWith2FA - пользователь с активным вторым фактором (TOTP).
func userWith2FA() dto.User2FA {
	return dto.User2FA{
		ID:        uuid.New(),
		Email:     "user@example.com",
		Action2FA: secureoperation.ConfirmAction{Method: confirmmethod.TOTP, MaxAttempts: 3, Expiry: time.Minute},
	}
}

// userWithout2FA - пользователь без второго фактора.
func userWithout2FA() dto.User2FA {
	return dto.User2FA{ID: uuid.New(), Email: "user@example.com"}
}

func (s *FactorySuite) TestChangeEmailCreate() {
	s.Run("without 2fa - single action", func() {
		s.expectGenerators()

		f := unit.NewChangeEmail(s.tokenGen, s.codeGen)

		op, err := f.Create(userWithout2FA(), "new@example.com")
		s.Require().NoError(err)
		s.Equal(unit.NameConfirmChangeEmail, op.Name)
		s.Require().Len(op.Actions(), 1)

		var p dto.ChangeEmailOperation
		s.Require().NoError(json.Unmarshal(op.Payload, &p))
		s.Equal("new@example.com", p.NewEmail)
		s.Equal("user@example.com", p.Email)
	})

	s.Run("with 2fa - appends second action", func() {
		s.expectGenerators()

		f := unit.NewChangeEmail(s.tokenGen, s.codeGen)

		op, err := f.Create(userWith2FA(), "new@example.com")
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
	})

	s.Run("token generator error", func() {
		wantErr := errors.New("token failed")
		s.tokenGen.EXPECT().GenToken().Return("", wantErr).AnyTimes()
		s.codeGen.EXPECT().GenCodeWithHash().Return("123456", "hashed-code", nil).AnyTimes()

		f := unit.NewChangeEmail(s.tokenGen, s.codeGen)

		_, err := f.Create(userWithout2FA(), "new@example.com")
		s.Require().ErrorIs(err, wantErr)
	})
}

func (s *FactorySuite) TestChangePasswordCreate() {
	s.expectGenerators()
	s.codeGen.EXPECT().HashedSecret("new-password").Return("hashed-pw", nil).AnyTimes()

	f := unit.NewChangePassword(s.tokenGen, s.codeGen)

	op, err := f.Create(userWithout2FA(), "new-password")
	s.Require().NoError(err)
	s.Equal(unit.NameConfirmChangePassword, op.Name)

	var p dto.ChangePasswordOperation
	s.Require().NoError(json.Unmarshal(op.Payload, &p))
	s.Equal("hashed-pw", p.NewPassword) // хранится хеш, не открытый пароль
	s.Equal("user@example.com", p.Email)

	op2fa, err := f.Create(userWith2FA(), "new-password")
	s.Require().NoError(err)
	s.Require().Len(op2fa.Actions(), 2)
}

func (s *FactorySuite) TestChangePhoneCreate() {
	s.expectGenerators()

	f := unit.NewChangePhone(s.tokenGen, s.codeGen)

	op, err := f.Create(userWithout2FA(), "79991234567")
	s.Require().NoError(err)
	s.Equal(unit.NameConfirmChangePhone, op.Name)

	var p dto.ChangePhoneOperation
	s.Require().NoError(json.Unmarshal(op.Payload, &p))
	s.Equal(uint64(79991234567), p.NewPhone)
	s.Equal("user@example.com", p.Email)

	op2fa, err := f.Create(userWith2FA(), "79991234567")
	s.Require().NoError(err)
	s.Require().Len(op2fa.Actions(), 2)
}

func (s *FactorySuite) TestChangePhoneCreateInvalidPhone() {
	s.expectGenerators()

	f := unit.NewChangePhone(s.tokenGen, s.codeGen)

	// "0000000000" проходит tag_phone на границе ввода, поэтому негодным его признаёт фабрика,
	// и признаёт именно пользовательской ошибкой, а не внутренней
	for _, phone := range []string{"not-a-number", "0000000000", "0"} {
		_, err := f.Create(userWithout2FA(), phone)
		s.Require().ErrorIs(err, contactaddress.ErrPhoneIsInvalid, "phone: %s", phone)
	}
}

func (s *FactorySuite) TestChangeTOTPCreate() {
	s.expectGenerators()
	s.secretGen.EXPECT().GenerateSecret(gomock.Any()).Return("TOTPSECRET", nil).AnyTimes()

	f := unit.NewChangeTOTP(s.tokenGen, s.codeGen, s.secretGen)

	op, err := f.Create(userWithout2FA())
	s.Require().NoError(err)
	s.Equal(unit.NameConfirmChangeTOTP, op.Name)

	var p dto.ChangeTOTPOperation
	s.Require().NoError(json.Unmarshal(op.Payload, &p))
	s.Equal("TOTPSECRET", p.Secret)
	s.Equal("user@example.com", p.Email)

	op2fa, err := f.Create(userWith2FA())
	s.Require().NoError(err)
	s.Require().Len(op2fa.Actions(), 2)
}

func (s *FactorySuite) TestCreateUserCreate() {
	registeredIP := mrtype.NewIP(netip.MustParseAddr("203.0.113.7"))

	s.Run("new user - single email action, nil user id", func() {
		s.expectGenerators()

		f := unit.NewCreateUser("shop", "customer", s.tokenGen, s.codeGen)

		// для нового email usecase передаёт пустой User2FA
		op, err := f.Create(dto.User2FA{}, "en", "Europe/Moscow", contactaddress.NewEmail("user@example.com"), registeredIP)
		s.Require().NoError(err)
		s.Equal(unit.NameConfirmCreateUser, op.Name)
		s.Equal(uuid.Nil, op.UserID)
		s.Require().Len(op.Actions(), 1)

		var p dto.CreateUserOperation
		s.Require().NoError(json.Unmarshal(op.Payload, &p))
		s.Equal("shop", p.Realm)
		s.Equal("customer", p.UserKind)
		s.Equal("en", p.LangCode)
		s.Equal("Europe/Moscow", p.TimeZone)
		s.Equal("user@example.com", p.Email)
		s.Equal(registeredIP, p.RegisteredIP)
	})

	s.Run("existing user with 2fa - appends second action and binds user id", func() {
		s.expectGenerators()

		f := unit.NewCreateUser("shop", "customer", s.tokenGen, s.codeGen)

		user2FA := userWith2FA()

		op, err := f.Create(user2FA, "en", "Europe/Moscow", contactaddress.NewEmail("user@example.com"), registeredIP)
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.Equal(user2FA.ID, op.UserID)
	})

	s.Run("existing user without 2fa - single email action", func() {
		s.expectGenerators()

		f := unit.NewCreateUser("shop", "customer", s.tokenGen, s.codeGen)

		user2FA := userWithout2FA()

		op, err := f.Create(user2FA, "en", "Europe/Moscow", contactaddress.NewEmail("user@example.com"), registeredIP)
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 1)
		s.Equal(user2FA.ID, op.UserID)
	})
}

func (s *FactorySuite) TestDisable2FACreate() {
	s.Run("with active 2fa", func() {
		s.expectGenerators()

		f := unit.NewDisable2FA(s.tokenGen, s.codeGen)

		op, err := f.Create(userWith2FA())
		s.Require().NoError(err)
		s.Equal(unit.NameConfirmDisable2FA, op.Name)
		s.Require().Len(op.Actions(), 2)

		var p dto.Disable2FAOperation
		s.Require().NoError(json.Unmarshal(op.Payload, &p))
		s.Equal("user@example.com", p.Email)
	})

	s.Run("already disabled fails", func() {
		s.expectGenerators()

		f := unit.NewDisable2FA(s.tokenGen, s.codeGen)

		_, err := f.Create(userWithout2FA())
		s.Require().ErrorContains(err, "2fa already disabled")
	})
}

func (s *FactorySuite) TestRegenerateRecoveryCreate() {
	s.Run("with active 2fa", func() {
		s.expectGenerators()

		f := unit.NewRegenerateRecovery(s.tokenGen, s.codeGen)

		op, err := f.Create(userWith2FA())
		s.Require().NoError(err)
		s.Equal(unit.NameConfirmRegenerateRecovery, op.Name)
		s.Require().Len(op.Actions(), 2) // email + текущий 2FA

		var p dto.OperationWithUserEmail
		s.Require().NoError(json.Unmarshal(op.Payload, &p))
		s.Equal("user@example.com", p.Email)
	})

	s.Run("without 2fa fails", func() {
		s.expectGenerators()

		f := unit.NewRegenerateRecovery(s.tokenGen, s.codeGen)

		_, err := f.Create(userWithout2FA())
		s.Require().ErrorContains(err, "2fa is not enabled")
	})
}

func (s *FactorySuite) TestAuthorizeUserCreate() {
	s.expectGenerators()

	f := unit.NewAuthorizeUser(s.tokenGen, s.codeGen)

	op, err := f.Create(userWithout2FA(), "shop", "en", contactaddress.NewEmail("login@example.com"))
	s.Require().NoError(err)
	s.Equal(unit.NameAuthorizeUser, op.Name)

	var p dto.AuthorizeUserOperation
	s.Require().NoError(json.Unmarshal(op.Payload, &p))
	s.Equal("shop", p.Realm)
	s.Equal("en", p.LangCode)
}

func (s *FactorySuite) TestAuthorizeUserCreatePhoneConvertedToEmail() {
	s.expectGenerators()

	// confirmPhoneByEmail по умолчанию true: телефонный логин подтверждается по email.
	f := unit.NewAuthorizeUser(s.tokenGen, s.codeGen)

	op, err := f.Create(userWith2FA(), "shop", "en", contactaddress.NewPhone("79991234567"))
	s.Require().NoError(err)
	s.Require().Len(op.Actions(), 2)

	firstAction, ok := op.FirstAction()
	s.Require().True(ok)
	s.Equal(confirmmethod.Email, firstAction.Method)
}

func (s *FactorySuite) TestAuthorizeUserCreatePhoneLoginWithOptions() {
	s.expectGenerators()

	f := unit.NewAuthorizeUser(
		s.tokenGen,
		s.codeGen,
		unit.WithAuthorizeUserConfirmByEmailOpts(action.WithMaxAttempts(5)),
		unit.WithAuthorizeUserConfirmByPhoneOpts(action.WithMaxAttempts(5)),
		unit.WithAuthorizeUserConfirmPhoneByEmail(false),
	)

	op, err := f.Create(userWithout2FA(), "shop", "en", contactaddress.NewPhone("79991234567"))
	s.Require().NoError(err)

	firstAction, ok := op.FirstAction()
	s.Require().True(ok)
	s.Equal(confirmmethod.Phone, firstAction.Method)
}
