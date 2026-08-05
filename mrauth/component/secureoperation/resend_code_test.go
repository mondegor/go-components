package secureoperation_test

import (
	"errors"
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

type ResendCodeSuite struct {
	suite.Suite

	ctrl     *gomock.Controller
	tokenGen *mock.MockTokenGenerator
	codeGen  *mock.MockCodeGenerator
	svc      *secureoperation.ResendCode
}

func TestResendCodeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ResendCodeSuite))
}

func (s *ResendCodeSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.tokenGen = mock.NewMockTokenGenerator(s.ctrl)
	s.codeGen = mock.NewMockCodeGenerator(s.ctrl)
	s.svc = secureoperation.NewResendCode(s.tokenGen, s.codeGen)

	s.codeGen.EXPECT().GenCodeWithHash().Return("123456", "123456", nil).AnyTimes()
}

// openedEmailOp - создаёт sendable-операцию (Email) в статусе Opened, готовую к
// повторной отправке кода (есть оставшиеся попытки, ResendsAt в прошлом).
func (s *ResendCodeSuite) openedEmailOp() secureoperation_model.SecureOperation {
	op := secureoperation_model.SecureOperation{
		Token:             "token",
		Name:              "name1",
		UserID:            uuid.New(),
		RemainingAttempts: 3,
		RemainingResends:  5,
		ResendsAt:         time.Now().Add(-time.Minute),
		Status:            operationstatus.Opened,
		ExpiresAt:         time.Now().Add(10 * time.Minute),
	}
	s.Require().NoError(secureoperation_model.WakeUp(&op, []secureoperation_model.ConfirmAction{
		{
			Method:        confirmmethod.Email,
			MaxAttempts:   3,
			MaxResends:    5,
			MinResendTime: 5 * time.Minute,
			Expiry:        10 * time.Minute,
			Address:       "u@e",
		},
	}))

	return op
}

func (s *ResendCodeSuite) TestPrepareSuccess() {
	s.tokenGen.EXPECT().GenToken().Return("new-token", nil)

	out, err := s.svc.Prepare(s.openedEmailOp())
	s.Require().NoError(err)
	s.Equal("new-token", out.Token)

	action, ok := out.FirstAction()
	s.Require().True(ok)
	s.Equal("123456", action.ConfirmCode)
}

func (s *ResendCodeSuite) TestPrepareTokenGeneratorError() {
	wantErr := errors.New("token generation failed")
	s.tokenGen.EXPECT().GenToken().Return("", wantErr)

	_, err := s.svc.Prepare(s.openedEmailOp())
	s.Require().ErrorIs(err, wantErr)
}

// TestPrepareBusinessErrorsKeepOperation - временный троттл и окончательно израсходованные
// отправки - бизнес-результат, а не сбой: вызывающий отдаёт клиенту актуальные счётчики
// операции вместе с ошибкой, поэтому операция обязана вернуться непустой. Нулевая операция
// здесь стоила бы клиенту пустого operation_state, а журналу - пустого имени операции.
func (s *ResendCodeSuite) TestPrepareBusinessErrorsKeepOperation() {
	for _, tt := range []struct {
		name    string
		prepare func(op *secureoperation_model.SecureOperation)
		wantErr error
	}{
		{
			name: "ещё рано",
			prepare: func(op *secureoperation_model.SecureOperation) {
				op.ResendsAt = time.Now().Add(time.Minute)
			},
			wantErr: secureoperation_model.ErrSendingNewMessagesIsTemporarilyRestricted,
		},
		{
			name: "отправки исчерпаны",
			prepare: func(op *secureoperation_model.SecureOperation) {
				op.RemainingResends = 0
			},
			wantErr: secureoperation_model.ErrNoAttemptsToResendCode,
		},
	} {
		s.Run(tt.name, func() {
			s.tokenGen.EXPECT().GenToken().Return("new-token", nil)

			op := s.openedEmailOp()
			tt.prepare(&op)

			out, err := s.svc.Prepare(op)
			s.Require().ErrorIs(err, tt.wantErr)
			s.Equal(op.Token, out.Token, "операция должна вернуться вместе с ошибкой")
			s.Equal(op.Name, out.Name)
		})
	}
}

// TestPrepareNonSendableActionFails - повторная отправка по 2FA-действию (TOTP/пароль)
// неприменима: отправлять нечего. Это отказ клиенту, а не сбой, поэтому отдаётся отдельным
// пользовательским сентинелом; операция с ним не возвращается - счётчики отправок
// у такого действия всё равно не заполняются.
func (s *ResendCodeSuite) TestPrepareNonSendableActionFails() {
	s.tokenGen.EXPECT().GenToken().Return("new-token", nil).AnyTimes()

	op := secureoperation_model.SecureOperation{
		Token:             "token",
		Name:              "name1",
		UserID:            uuid.New(),
		RemainingAttempts: 3,
		Status:            operationstatus.Opened,
		ExpiresAt:         time.Now().Add(10 * time.Minute),
	}
	s.Require().NoError(secureoperation_model.WakeUp(&op, []secureoperation_model.ConfirmAction{
		{
			Method:      confirmmethod.TOTP,
			MaxAttempts: 3,
			Expiry:      10 * time.Minute,
		},
	}))

	_, err := s.svc.Prepare(op)
	s.Require().ErrorIs(err, secureoperation_model.ErrResendCodeIsNotSupported)
	s.Require().NotErrorIs(err, secureoperation_model.ErrNoAttemptsToResendCode)
}

func (s *ResendCodeSuite) TestPrepareNotOpenedFails() {
	s.tokenGen.EXPECT().GenToken().Return("new-token", nil).AnyTimes()

	confirmed := secureoperation_model.SecureOperation{
		Token:     "token",
		Name:      "name1",
		UserID:    uuid.New(),
		Status:    operationstatus.Confirmed,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	s.Require().NoError(secureoperation_model.WakeUp(&confirmed, nil))

	_, err := s.svc.Prepare(confirmed)
	s.Require().ErrorIs(err, secureoperation_model.ErrOperationAlreadyConfirmed)
}
