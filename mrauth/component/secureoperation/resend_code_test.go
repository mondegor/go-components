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
