package secureoperation_test

import (
	"time"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	secureoperation_model "github.com/mondegor/go-components/mrauth/model/secureoperation"
)

func emailConfirmAction(code string) secureoperation_model.ConfirmAction {
	return secureoperation_model.ConfirmAction{
		Method:        confirmmethod.Email,
		MaxAttempts:   3,
		MaxResends:    5,
		MinResendTime: 5 * time.Minute,
		Expiry:        10 * time.Minute,
		Address:       "u@e",
		ConfirmCode:   code,
	}
}

// newOpWithActions - создаёт операцию в статусе Opened с указанными действиями.
func (s *ConfirmCodeSuite) newOpWithActions(actions ...secureoperation_model.ConfirmAction) secureoperation_model.SecureOperation {
	op, err := secureoperation_model.NewOperation("token", "name1", uuid.New(), actions, nil)
	s.Require().NoError(err)

	return op
}

func (s *ConfirmCodeSuite) TestEmailCorrectCodeConfirms() {
	s.expectGenerators("tok", "code")

	op := s.newOpWithActions(emailConfirmAction("secret1"))

	out, commit, err := s.svc.Prepare(s.ctx, op, "secret1")
	s.Require().NoError(err)
	s.True(out.Is(operationstatus.Confirmed))
	s.Nil(commit)
}

func (s *ConfirmCodeSuite) TestEmailWrongCodeRejected() {
	s.expectGenerators("tok", "code")

	op := s.newOpWithActions(emailConfirmAction("secret1"))

	out, commit, err := s.svc.Prepare(s.ctx, op, "wrong")
	s.Require().ErrorIs(err, secureoperation_model.ErrConfirmCodeIsIncorrect)
	s.False(out.Is(operationstatus.Confirmed))
	s.Nil(commit)
}

func (s *ConfirmCodeSuite) TestFirstOfTwoActionsGeneratesNextCode() {
	// значения генераторов выбраны так, чтобы было видно: код следующего действия
	// и токен операции берутся именно из них
	s.expectGenerators("new-token", "new-code")

	op := s.newOpWithActions(emailConfirmAction("secret1"), emailConfirmAction("secret2"))

	out, _, err := s.svc.Prepare(s.ctx, op, "secret1")
	s.Require().NoError(err)
	s.False(out.Is(operationstatus.Confirmed))
	s.Equal("new-token", out.Token)

	action, ok := out.FirstAction()
	s.Require().True(ok)
	s.Equal("new-code", action.ConfirmCode)
}
