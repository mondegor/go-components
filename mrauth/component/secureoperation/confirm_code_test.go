package secureoperation_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/component/secureoperation/mock"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	secureoperation_model "github.com/mondegor/go-components/mrauth/model/secureoperation"
)

//go:generate mockgen -source=confirm_code.go -destination=mock/confirm_code.go -package=mock
//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth TokenGenerator,CodeGenerator

// ConfirmCodeSuite - общий набор для тестов ConfirmCode; методы объявлены также
// в confirm_code_sendable_test.go (проверка подтверждения по контактному адресу).
type ConfirmCodeSuite struct {
	suite.Suite

	ctrl     *gomock.Controller
	ctx      context.Context
	tokenGen *mock.MockTokenGenerator
	codeGen  *mock.MockCodeGenerator
	verifier *mock.Mockauth2faVerifier
	svc      *secureoperation.ConfirmCode
}

func TestConfirmCodeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ConfirmCodeSuite))
}

func (s *ConfirmCodeSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.tokenGen = mock.NewMockTokenGenerator(s.ctrl)
	s.codeGen = mock.NewMockCodeGenerator(s.ctrl)
	s.verifier = mock.NewMockauth2faVerifier(s.ctrl)
	s.svc = secureoperation.NewConfirmCode(s.tokenGen, s.codeGen, s.verifier)
}

// expectGenerators - настраивает генераторы токена и кода следующего действия.
// Хеш кода в тестах равен самому коду, поэтому сравнение сводится к равенству строк.
func (s *ConfirmCodeSuite) expectGenerators(token, code string) {
	s.tokenGen.EXPECT().GenToken().Return(token, nil).AnyTimes()
	s.codeGen.EXPECT().GenCodeWithHash().Return(code, code, nil).AnyTimes()
	s.codeGen.EXPECT().
		CompareSecretAndHash(gomock.Any(), gomock.Any()).
		DoAndReturn(func(secret, hashedSecret string) (bool, error) {
			return secret == hashedSecret, nil
		}).
		AnyTimes()
}

// newOpWithSingleTOTPAction - создаёт операцию в статусе Opened с одним
// действием method=TOTP для указанного пользователя.
func (s *ConfirmCodeSuite) newOpWithSingleTOTPAction(userID uuid.UUID) secureoperation_model.SecureOperation {
	op, err := secureoperation_model.NewOperation(
		"token",
		"name1",
		userID,
		[]secureoperation_model.ConfirmAction{
			{
				Method:      confirmmethod.TOTP,
				MaxAttempts: 3,
				Expiry:      10 * time.Minute,
			},
		},
		nil,
	)
	s.Require().NoError(err)

	return op
}

func (s *ConfirmCodeSuite) TestTOTPVerifiedNoConsume() {
	userID := uuid.New()

	s.verifier.EXPECT().
		Verify(gomock.Any(), userID, confirmmethod.TOTP, "123456").
		Return(true, nil, nil)

	out, commitConfirmed, err := s.svc.Prepare(s.ctx, s.newOpWithSingleTOTPAction(userID), "123456")
	s.Require().NoError(err)
	s.True(out.Is(operationstatus.Confirmed))
	s.Nil(commitConfirmed)
}

func (s *ConfirmCodeSuite) TestTOTPVerifiedWithConsume() {
	called := false
	consume := func(_ context.Context) error {
		called = true

		return nil
	}

	s.verifier.EXPECT().
		Verify(gomock.Any(), gomock.Any(), confirmmethod.TOTP, gomock.Any()).
		Return(true, consume, nil)

	out, commitConfirmed, err := s.svc.Prepare(s.ctx, s.newOpWithSingleTOTPAction(uuid.New()), "recovery")
	s.Require().NoError(err)
	s.True(out.Is(operationstatus.Confirmed))
	s.Require().NotNil(commitConfirmed)

	s.Require().NoError(commitConfirmed(s.ctx))
	s.True(called)
}

func (s *ConfirmCodeSuite) TestTOTPVerifierRejects() {
	s.verifier.EXPECT().
		Verify(gomock.Any(), gomock.Any(), confirmmethod.TOTP, gomock.Any()).
		Return(false, nil, nil)

	out, commitConfirmed, err := s.svc.Prepare(s.ctx, s.newOpWithSingleTOTPAction(uuid.New()), "bad")
	s.Require().ErrorIs(err, secureoperation_model.ErrConfirmCodeIsIncorrect)
	s.False(out.Is(operationstatus.Confirmed))
	s.Nil(commitConfirmed)
}

func (s *ConfirmCodeSuite) TestTOTPVerifierError() {
	wantErr := secureoperation_model.ErrOperationAlreadyExpired

	s.verifier.EXPECT().
		Verify(gomock.Any(), gomock.Any(), confirmmethod.TOTP, gomock.Any()).
		Return(false, nil, wantErr)

	out, commitConfirmed, err := s.svc.Prepare(s.ctx, s.newOpWithSingleTOTPAction(uuid.New()), "any")
	s.Require().ErrorIs(err, wantErr)
	s.False(out.Is(operationstatus.Confirmed))
	s.Nil(commitConfirmed)
}
