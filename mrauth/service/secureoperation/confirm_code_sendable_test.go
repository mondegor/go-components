package secureoperation_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
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
	op, err := secureoperation_model.NewOperation("token", operationtype.ChangePhone, uuid.New(), actions, nil)
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
	s.Require().ErrorIs(err, mrauth.ErrConfirmCodeIsIncorrect)
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

// recoveryConfirmAction - завершающее звено цепочки, принимающее аварийный код.
func recoveryConfirmAction() secureoperation_model.ConfirmAction {
	return secureoperation_model.ConfirmAction{
		Method:      confirmmethod.Recovery,
		MaxAttempts: 3,
		Expiry:      10 * time.Minute,
	}
}

// totpConfirmAction - звено второго фактора цепочки "второй фактор -> аварийный код".
func totpConfirmAction() secureoperation_model.ConfirmAction {
	return secureoperation_model.ConfirmAction{
		Method:      confirmmethod.TOTP,
		MaxAttempts: 3,
		Expiry:      10 * time.Minute,
	}
}

// TestRecoveryChainConfirms - вход по комбинации "второй фактор + аварийный код": оба звена
// не-sendable, письмо не генерится, а commit расхода кода отдаётся вызывающему на последнем
// звене - там же, где комбинация принимается целиком.
func (s *ConfirmCodeSuite) TestRecoveryChainConfirms() {
	s.expectGenerators("new-token", "new-code")

	consume := func(_ context.Context) error { return nil }

	s.verifier.EXPECT().
		Verify(gomock.Any(), gomock.Any(), confirmmethod.TOTP, false, "123456").
		Return(true, nil, nil)
	s.verifier.EXPECT().
		Verify(gomock.Any(), gomock.Any(), confirmmethod.Recovery, false, "AAAAABBBBB").
		Return(true, consume, nil)

	op := s.newOpWithActions(totpConfirmAction(), recoveryConfirmAction())

	out, commit, err := s.svc.Prepare(s.ctx, op, "123456")
	s.Require().NoError(err)
	s.False(out.Is(operationstatus.Confirmed))
	s.Nil(commit)

	next, ok := out.FirstAction()
	s.Require().True(ok)
	s.Equal(confirmmethod.Recovery, next.Method)

	out, commit, err = s.svc.Prepare(s.ctx, out, "AAAAABBBBB")
	s.Require().NoError(err)
	s.True(out.Is(operationstatus.Confirmed))
	s.Require().NotNil(commit)
}

// TestRecoveryActionWrongCodeRejected - неподошедший аварийный код гасит попытку,
// а не операцию, и commit'а за собой не оставляет.
func (s *ConfirmCodeSuite) TestRecoveryActionWrongCodeRejected() {
	s.expectGenerators("tok", "code")

	s.verifier.EXPECT().
		Verify(gomock.Any(), gomock.Any(), confirmmethod.Recovery, false, "ZZZZZYYYYY").
		Return(false, nil, nil)

	op := s.newOpWithActions(recoveryConfirmAction())

	out, commit, err := s.svc.Prepare(s.ctx, op, "ZZZZZYYYYY")
	s.Require().ErrorIs(err, mrauth.ErrConfirmCodeIsIncorrect)
	s.False(out.Is(operationstatus.Confirmed))
	s.Nil(commit)
}

// TestTOTPStepCommitSurvivesNonFinalAction - в цепочке "второй фактор -> аварийный код"
// первое звено не последнее, но его commit (продвинутый TOTP-шаг) обязан дойти до вызывающего:
// иначе шаг не зафиксируется и тот же код можно предъявить повторно.
func (s *ConfirmCodeSuite) TestTOTPStepCommitSurvivesNonFinalAction() {
	s.expectGenerators("new-token", "new-code")

	stepCommit := func(_ context.Context) error { return nil }

	s.verifier.EXPECT().
		Verify(gomock.Any(), gomock.Any(), confirmmethod.TOTP, false, "123456").
		Return(true, stepCommit, nil)

	op := s.newOpWithActions(totpConfirmAction(), recoveryConfirmAction())

	out, commit, err := s.svc.Prepare(s.ctx, op, "123456")
	s.Require().NoError(err)
	s.False(out.Is(operationstatus.Confirmed))
	s.Require().NotNil(commit)
}
