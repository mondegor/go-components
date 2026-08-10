package unit_test

import (
	"encoding/json"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
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

// decoySelector - настоящий селектор подставного второго фактора с постоянным тестовым ключом:
// поведение заглушки проверяется вместе с ним, а не подменяется.
func (s *FactorySuite) decoySelector() *crypt.DecoyFactorSelector {
	selector, err := crypt.NewDecoyFactorSelector([]byte("test-decoy-key-0123456789abcdef0"), crypt.DefaultTOTPPercent())
	s.Require().NoError(err)

	return selector
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

		op, err := f.Create(userWithout2FA(), contactaddress.NewEmail("new@example.com"))
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

		op, err := f.Create(userWith2FA(), contactaddress.NewEmail("new@example.com"))
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
	})

	s.Run("token generator error", func() {
		wantErr := errors.New("token failed")
		s.tokenGen.EXPECT().GenToken().Return("", wantErr).AnyTimes()
		s.codeGen.EXPECT().GenCodeWithHash().Return("123456", "hashed-code", nil).AnyTimes()

		f := unit.NewChangeEmail(s.tokenGen, s.codeGen)

		_, err := f.Create(userWithout2FA(), contactaddress.NewEmail("new@example.com"))
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

	op, err := f.Create(userWithout2FA(), contactaddress.NewPhone("79991234567"))
	s.Require().NoError(err)
	s.Equal(unit.NameConfirmChangePhone, op.Name)

	var p dto.ChangePhoneOperation
	s.Require().NoError(json.Unmarshal(op.Payload, &p))
	s.Equal(uint64(79991234567), p.NewPhone)
	s.Equal("user@example.com", p.Email)

	op2fa, err := f.Create(userWith2FA(), contactaddress.NewPhone("79991234567"))
	s.Require().NoError(err)
	s.Require().Len(op2fa.Actions(), 2)
}

func (s *FactorySuite) TestChangePhoneCreateInvalidPhone() {
	s.expectGenerators()

	f := unit.NewChangePhone(s.tokenGen, s.codeGen)

	// такие значения отсекаются ещё тегом на границе ввода, поэтому пустой адрес,
	// который возвращает на них NewPhone, для фабрики - нарушение инварианта
	for _, phone := range []string{"not-a-number", "0000000000", "0"} {
		_, err := f.Create(userWithout2FA(), contactaddress.NewPhone(phone))
		s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData, "phone: %s", phone)
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
		s.Require().ErrorIs(err, mrauth.ErrAuth2FAIsDisabled)
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
		s.Require().ErrorIs(err, mrauth.ErrAuth2FAIsDisabled)
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

	// confirmPhoneByEmail по умолчанию true: телефонный логин подтверждается по email
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

// TestChangeEmailConfirmsCurrentAddress - операция смены email собирает доказательства владения
// аккаунтом, поэтому код подтверждения уходит на текущий адрес пользователя, а не на новый:
// владение новым адресом подтверждается отдельным шагом сценария смены адреса.
func (s *FactorySuite) TestChangeEmailConfirmsCurrentAddress() {
	s.expectGenerators()

	f := unit.NewChangeEmail(s.tokenGen, s.codeGen)

	op, err := f.Create(userWith2FA(), contactaddress.NewEmail("new@example.com"))
	s.Require().NoError(err)

	firstAction, ok := op.FirstAction()
	s.Require().True(ok)
	s.Equal(confirmmethod.Email, firstAction.Method)
	s.Equal("user@example.com", firstAction.Address)

	// новый адрес при этом сохраняется в payload - им пользуется обработчик применения операции
	var p dto.ChangeEmailOperation
	s.Require().NoError(json.Unmarshal(op.Payload, &p))
	s.Equal("new@example.com", p.NewEmail)
}

// TestRecoveryPolicy - у каждой операции свой набор допустимых комбинаций доказательств
// (спецификация 2FA, таблица комбинаций), и выражен он двумя способами: признаком AllowRecovery
// на завершающем звене (аварийный код вместо второго фактора) и отдельной цепочкой с завершающим
// звеном RECOVERY (аварийный код в дополнение ко второму фактору). Оба невидимы в нулевом
// значении, поэтому проверяются явно у каждой фабрики.
func (s *FactorySuite) TestRecoveryPolicy() {
	registeredIP := mrtype.NewIP(netip.MustParseAddr("203.0.113.7"))

	// вход: "email-код + пароль/TOTP" и "email-код + аварийный код" - одна и та же цепочка,
	// на последнем звене пользователь вводит то, что у него есть
	s.Run("authorize user - allowed instead of the second factor", func() {
		s.expectGenerators()

		f := unit.NewAuthorizeUser(s.tokenGen, s.codeGen)

		op, err := f.Create(userWith2FA(), "shop", "en", contactaddress.NewEmail("login@example.com"))
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.False(op.Actions()[0].AllowRecovery)
		s.True(op.Actions()[1].AllowRecovery)
	})

	// в S0 аварийных кодов нет вовсе, подставлять их некуда
	s.Run("authorize user without 2fa - not allowed", func() {
		s.expectGenerators()

		f := unit.NewAuthorizeUser(s.tokenGen, s.codeGen)

		op, err := f.Create(userWithout2FA(), "shop", "en", contactaddress.NewEmail("login@example.com"))
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 1)
		s.False(op.Actions()[0].AllowRecovery)
	})

	// снятие 2FA обязательно подтверждается email-кодом
	s.Run("disable 2fa - allowed instead of the second factor", func() {
		s.expectGenerators()

		f := unit.NewDisable2FA(s.tokenGen, s.codeGen)

		op, err := f.Create(userWith2FA())
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.False(op.Actions()[0].AllowRecovery)
		s.True(op.Actions()[1].AllowRecovery)
	})

	// перевыпуск заменяет сами аварийные коды, поэтому требует оба постоянных элемента
	s.Run("regenerate recovery - not allowed at all", func() {
		s.expectGenerators()

		f := unit.NewRegenerateRecovery(s.tokenGen, s.codeGen)

		op, err := f.Create(userWith2FA())
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.False(op.Actions()[0].AllowRecovery)
		s.False(op.Actions()[1].AllowRecovery)
	})

	// смена адреса обязательно подтверждается паролем/TOTP, поэтому "email-код + аварийный код"
	// отклоняется; комбинацию с аварийным кодом даёт только ChangeEmailByRecovery
	s.Run("change email - not allowed in the regular chain", func() {
		s.expectGenerators()

		f := unit.NewChangeEmail(s.tokenGen, s.codeGen)

		op, err := f.Create(userWith2FA(), contactaddress.NewEmail("new@example.com"))
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.False(op.Actions()[0].AllowRecovery)
		s.False(op.Actions()[1].AllowRecovery)
	})

	// прочие операции таблицей комбинаций не описаны и аварийный код не принимают
	s.Run("create user - not allowed", func() {
		s.expectGenerators()

		f := unit.NewCreateUser("shop", "customer", s.tokenGen, s.codeGen)

		op, err := f.Create(userWith2FA(), "en", "Europe/Moscow", contactaddress.NewEmail("user@example.com"), registeredIP)
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.False(op.Actions()[0].AllowRecovery)
		s.False(op.Actions()[1].AllowRecovery)
	})

	s.Run("change password - not allowed", func() {
		s.expectGenerators()
		s.codeGen.EXPECT().HashedSecret(gomock.Any()).Return("hashed-pw", nil).AnyTimes()

		f := unit.NewChangePassword(s.tokenGen, s.codeGen)

		op, err := f.Create(userWith2FA(), "new-password")
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.False(op.Actions()[0].AllowRecovery)
		s.False(op.Actions()[1].AllowRecovery)
	})

	s.Run("change totp - not allowed", func() {
		s.expectGenerators()
		s.secretGen.EXPECT().GenerateSecret(gomock.Any()).Return("TOTPSECRET", nil).AnyTimes()

		f := unit.NewChangeTOTP(s.tokenGen, s.codeGen, s.secretGen)

		op, err := f.Create(userWith2FA())
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.False(op.Actions()[0].AllowRecovery)
		s.False(op.Actions()[1].AllowRecovery)
	})

	s.Run("change phone - not allowed", func() {
		s.expectGenerators()

		f := unit.NewChangePhone(s.tokenGen, s.codeGen)

		op, err := f.Create(userWith2FA(), contactaddress.NewPhone("79991234567"))
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.False(op.Actions()[0].AllowRecovery)
		s.False(op.Actions()[1].AllowRecovery)
	})
}

// TestByRecoveryChains - цепочка "второй фактор -> аварийный код" для утративших доступ
// к почте: код подтверждения в ней не участвует, поэтому письмо не отправляется, а аварийный
// код стоит последним звеном - там же, где комбинация принимается целиком.
func (s *FactorySuite) TestByRecoveryChains() {
	s.Run("authorize user", func() {
		s.expectGenerators()

		f := unit.NewAuthorizeUserByRecovery(s.tokenGen, s.decoySelector())

		op, err := f.Create(userWith2FA(), "shop", "en")
		s.Require().NoError(err)
		s.Equal(unit.NameAuthorizeUser, op.Name) // то же имя: Opener вытесняет прежний вход
		s.Require().Len(op.Actions(), 2)
		s.Equal(confirmmethod.TOTP, op.Actions()[0].Method)
		s.Equal(confirmmethod.Recovery, op.Actions()[1].Method)
		s.False(op.Actions()[0].AllowRecovery)
	})

	s.Run("change email", func() {
		s.expectGenerators()

		f := unit.NewChangeEmailByRecovery(s.tokenGen)

		op, err := f.Create(userWith2FA(), contactaddress.NewEmail("new@example.com"))
		s.Require().NoError(err)
		s.Equal(unit.NameConfirmChangeEmail, op.Name) // то же имя: Opener вытесняет прежнюю смену
		s.Require().Len(op.Actions(), 2)
		s.Equal(confirmmethod.TOTP, op.Actions()[0].Method)
		s.Equal(confirmmethod.Recovery, op.Actions()[1].Method)

		// новый адрес по-прежнему едет в payload - им пользуется обработчик применения операции
		var p dto.ChangeEmailOperation
		s.Require().NoError(json.Unmarshal(op.Payload, &p))
		s.Equal("new@example.com", p.NewEmail)
	})

	// метод авторизованный: своё состояние 2FA вызывающий знает, скрывать его не от кого
	s.Run("change email without 2fa is rejected", func() {
		s.expectGenerators()

		f := unit.NewChangeEmailByRecovery(s.tokenGen)

		_, err := f.Create(userWithout2FA(), contactaddress.NewEmail("new@example.com"))
		s.Require().ErrorIs(err, mrauth.ErrAuth2FAIsDisabled)
	})
}

// TestByRecoveryActionOptions - завершающее звено аварийного кода настраивается наравне
// с остальными: аварийный код предъявляется вместо второго фактора, поэтому лимит попыток
// и срок жизни у него обязаны задаваться проводкой, а не оставаться на умолчаниях.
func (s *FactorySuite) TestByRecoveryActionOptions() {
	s.Run("authorize user", func() {
		s.expectGenerators()

		f := unit.NewAuthorizeUserByRecovery(
			s.tokenGen,
			s.decoySelector(),
			unit.WithAuthorizeUserByRecoveryConfirmByRecoveryOpts(
				action.WithMaxAttempts(7),
				action.WithExpiry(time.Hour),
			),
		)

		op, err := f.Create(userWith2FA(), "shop", "en")
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.Equal(confirmmethod.Recovery, op.Actions()[1].Method)
		s.Equal(int16(7), op.Actions()[1].MaxAttempts)
		s.Equal(time.Hour, op.Actions()[1].Expiry)
	})

	s.Run("change email", func() {
		s.expectGenerators()

		f := unit.NewChangeEmailByRecovery(
			s.tokenGen,
			action.WithMaxAttempts(7),
			action.WithExpiry(time.Hour),
		)

		op, err := f.Create(userWith2FA(), contactaddress.NewEmail("new@example.com"))
		s.Require().NoError(err)
		s.Require().Len(op.Actions(), 2)
		s.Equal(confirmmethod.Recovery, op.Actions()[1].Method)
		s.Equal(int16(7), op.Actions()[1].MaxAttempts)
		s.Equal(time.Hour, op.Actions()[1].Expiry)
	})
}

// TestAuthorizeUserByRecoveryDecoy - вход по аварийному коду для аккаунта без 2FA: цепочка
// строится такая же, с подставным вторым фактором. Иначе ответ метода выдавал бы состояние
// аккаунта кому угодно, знающему логин, - метод-то гостевой.
func (s *FactorySuite) TestAuthorizeUserByRecoveryDecoy() {
	s.expectGenerators()

	f := unit.NewAuthorizeUserByRecovery(s.tokenGen, s.decoySelector())

	user := userWithout2FA()

	op, err := f.Create(user, "shop", "en")
	s.Require().NoError(err)
	s.Require().Len(op.Actions(), 2)
	s.Contains(
		[]confirmmethod.Enum{confirmmethod.Password, confirmmethod.TOTP},
		op.Actions()[0].Method,
	)
	s.Equal(confirmmethod.Recovery, op.Actions()[1].Method)

	// выбор подставного фактора постоянен для пользователя: иначе повторный вызов
	// с тем же логином отдавал бы разные типы и сам выдавал бы заглушку
	again, err := f.Create(user, "shop", "en")
	s.Require().NoError(err)
	s.Equal(op.Actions()[0].Method, again.Actions()[0].Method)
}

// TestAuthorizeUserByRecoveryUsesRealFactor - аккаунту с включённой 2FA звено берётся как есть,
// а не пересобирается селектором: подстановка предназначена только тем, у кого второго фактора
// нет, и подменять ею настоящий фактор значило бы сделать операцию неподтверждаемой.
func (s *FactorySuite) TestAuthorizeUserByRecoveryUsesRealFactor() {
	s.expectGenerators()

	f := unit.NewAuthorizeUserByRecovery(s.tokenGen, s.decoySelector())

	user := userWith2FA()

	op, err := f.Create(user, "shop", "en")
	s.Require().NoError(err)
	s.Require().Len(op.Actions(), 2)
	s.Equal(user.Action2FA, op.Actions()[0])
}

// TestAuthorizeUserByRecoveryDecoyMatchesRealChain - подставная цепочка обязана совпадать
// с настоящей не только методом, но и лимитом попыток со сроком жизни: они уезжают клиенту
// как remaining_attempts и expires_in, и по расхождению состояние 2FA читается одним гостевым
// запросом - никаких доказательств для этого не нужно.
//
// Совпадение обеспечивает проводка: подставное звено собирается тем же набором настроек,
// что и настоящее (см. confirm2faOpts в wire/mrauth/infra/pub/unit_auth.go). Тест держит
// само требование - здесь оба звена строятся одним и тем же factorOpts.
func (s *FactorySuite) TestAuthorizeUserByRecoveryDecoyMatchesRealChain() {
	s.expectGenerators()

	factorOpts := []action.Option{
		action.WithMaxAttempts(5),
		action.WithExpiry(30 * time.Minute),
	}

	real2FA, err := action.NewConfirmBy2fa(factorOpts, factorOpts).Create(auth2fatype.TOTP, "TOTPSECRET")
	s.Require().NoError(err)

	f := unit.NewAuthorizeUserByRecovery(
		s.tokenGen,
		s.decoySelector(),
		unit.WithAuthorizeUserByRecoveryConfirmByPasswordOpts(factorOpts...),
		unit.WithAuthorizeUserByRecoveryConfirmByTOTPOpts(factorOpts...),
	)

	withReal, err := f.Create(
		dto.User2FA{ID: uuid.New(), Email: "user@example.com", Action2FA: real2FA},
		"shop",
		"en",
	)
	s.Require().NoError(err)

	withDecoy, err := f.Create(userWithout2FA(), "shop", "en")
	s.Require().NoError(err)

	s.Equal(withReal.RemainingAttempts, withDecoy.RemainingAttempts)
	s.WithinDuration(withReal.ExpiresAt, withDecoy.ExpiresAt, time.Second)

	// повторные отправки не применимы ни там, ни там: первое звено обеих цепочек не sendable
	s.Equal(withReal.RemainingResends, withDecoy.RemainingResends)
}
