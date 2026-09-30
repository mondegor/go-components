package secureoperation_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	secureoperation_model "github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/service/secureoperation"
	"github.com/mondegor/go-components/mrauth/service/secureoperation/mock"
)

//go:generate mockgen -source=confirm_code.go -destination=mock/confirm_code.go -package=mock

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
	// новый токен выпускается той же длины, что текущий токен операции ("token")
	s.tokenGen.EXPECT().GenToken(len("token")).Return(token, nil).AnyTimes()
	s.codeGen.EXPECT().GenCodeWithHash(gomock.Any()).Return(code, code, nil).AnyTimes()
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
		operationtype.ChangePhone,
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

// prepareAsOwner - вызывает Prepare от имени владельца операции, как это делает вызывающий
// после dto.ActorMeta.WithUser.
func (s *ConfirmCodeSuite) prepareAsOwner(
	op secureoperation_model.SecureOperation,
	confirmCode string,
) (secureoperation_model.SecureOperation, func(ctx context.Context) error, error) {
	return s.svc.Prepare(s.ctx, dto.ActorMeta{UserID: op.UserID}, op, confirmCode)
}

// TestActorNotOwnerRejected - actor, не совпадающий с владельцем операции, - нарушение контракта
// вызывающего: иначе проверялась бы 2FA другого пользователя. Верификатор при этом не дёргается.
func (s *ConfirmCodeSuite) TestActorNotOwnerRejected() {
	s.verifier.EXPECT().Verify(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	for _, actorUserID := range []uuid.UUID{uuid.Nil, uuid.New()} {
		out, commitConfirmed, err := s.svc.Prepare(
			s.ctx,
			dto.ActorMeta{UserID: actorUserID},
			s.newOpWithSingleTOTPAction(uuid.New()),
			"123456",
		)
		s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData)
		s.False(out.Is(operationstatus.Confirmed))
		s.Nil(commitConfirmed)
	}
}

// TestEmptyConfirmCodeRejected - пустой секрет сюда попасть не может: вызывающий отсекает его
// до Prepare. Поэтому это нарушение контракта, а не пользовательский ввод, и отдаётся внутренней
// ошибкой, а не «введено неверно»; верификатор при этом не дёргается.
func (s *ConfirmCodeSuite) TestEmptyConfirmCodeRejected() {
	s.verifier.EXPECT().Verify(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	out, commitConfirmed, err := s.prepareAsOwner(s.newOpWithSingleTOTPAction(uuid.New()), "")
	s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData)
	s.Require().NotErrorIs(err, mrauth.ErrConfirmCodeIsIncorrect)
	s.False(out.Is(operationstatus.Confirmed))
	s.Nil(commitConfirmed)
}

func (s *ConfirmCodeSuite) TestTOTPVerifiedNoConsume() {
	userID := uuid.New()
	// клиент, подтверждающий операцию, передаётся верификатору как есть
	actor := dto.ActorMeta{UserID: userID, UserAgent: "test-agent"}

	s.verifier.EXPECT().
		Verify(gomock.Any(), actor, confirmmethod.TOTP, false, "123456").
		Return(true, nil, nil)

	out, commitConfirmed, err := s.svc.Prepare(s.ctx, actor, s.newOpWithSingleTOTPAction(userID), "123456")
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
		Verify(gomock.Any(), gomock.Any(), confirmmethod.TOTP, false, gomock.Any()).
		Return(true, consume, nil)

	out, commitConfirmed, err := s.prepareAsOwner(s.newOpWithSingleTOTPAction(uuid.New()), "recovery")
	s.Require().NoError(err)
	s.True(out.Is(operationstatus.Confirmed))
	s.Require().NotNil(commitConfirmed)

	s.Require().NoError(commitConfirmed(s.ctx))
	s.True(called)
}

func (s *ConfirmCodeSuite) TestTOTPVerifierRejects() {
	s.verifier.EXPECT().
		Verify(gomock.Any(), gomock.Any(), confirmmethod.TOTP, false, gomock.Any()).
		Return(false, nil, nil)

	out, commitConfirmed, err := s.prepareAsOwner(s.newOpWithSingleTOTPAction(uuid.New()), "bad")
	s.Require().ErrorIs(err, mrauth.ErrConfirmCodeIsIncorrect)
	s.False(out.Is(operationstatus.Confirmed))
	s.Nil(commitConfirmed)
}

func (s *ConfirmCodeSuite) TestTOTPVerifierError() {
	wantErr := mrauth.ErrOperationAlreadyExpired

	s.verifier.EXPECT().
		Verify(gomock.Any(), gomock.Any(), confirmmethod.TOTP, false, gomock.Any()).
		Return(false, nil, wantErr)

	out, commitConfirmed, err := s.prepareAsOwner(s.newOpWithSingleTOTPAction(uuid.New()), "any")
	s.Require().ErrorIs(err, wantErr)
	s.False(out.Is(operationstatus.Confirmed))
	s.Nil(commitConfirmed)
}

// Test2FADisabledLooksLikeMiss - 2FA у пользователя нет: либо цепочка подставная (построена аккаунту
// с выключенной 2FA), либо 2FA сняли уже после создания операции. Отдельного ответа ни у того,
// ни у другого случая быть не может: метод гостевой, и по такому ответу состояние 2FA аккаунта
// читалось бы одним запросом. Отсюда единый ответ, см.
// contracts/mrauth/paths/v1_operation_confirm.yaml.
func (s *ConfirmCodeSuite) Test2FADisabledLooksLikeMiss() {
	s.verifier.EXPECT().
		Verify(gomock.Any(), gomock.Any(), confirmmethod.TOTP, false, gomock.Any()).
		Return(false, nil, mrauth.ErrAuth2FAIsDisabled)

	out, commitConfirmed, err := s.prepareAsOwner(s.newOpWithSingleTOTPAction(uuid.New()), "123456")
	s.Require().ErrorIs(err, mrauth.ErrConfirmCodeIsIncorrect)
	s.Require().NotErrorIs(err, mrauth.ErrAuth2FAIsDisabled)
	s.False(out.Is(operationstatus.Confirmed))
	s.Nil(commitConfirmed)
}
