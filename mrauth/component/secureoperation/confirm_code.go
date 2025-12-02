package secureoperation

import (
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

type (
	// ConfirmCode - comment struct.
	ConfirmCode struct {
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewConfirmCode - создаёт объект OperationFactory.
func NewConfirmCode(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
) *ConfirmCode {
	return &ConfirmCode{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
	}
}

// Prepare - comments method.
func (o *ConfirmCode) Prepare(op entity.SecureOperation, confirmCode string) (entity.SecureOperation, error) {
	if time.Now().After(op.ExpiresAt) {
		return entity.SecureOperation{}, mrauth.ErrOperationAlreadyExpired.New()
	}

	// if item.Payload["audience"] == "" {
	// 	return 0, errors.New("invalid operation token")
	// }
	//
	// if item.Payload["visitor_id"] == "" {
	// 	return 0, errors.New("invalid operation token")
	// }

	if op.Status != operationstatus.Opened {
		return entity.SecureOperation{}, mrauth.ErrOperationAlreadyConfirmed.New() // operation is not opened
	}

	confirmingAction, err := op.NextNotConfirmedAction()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	if op.RemainingAttempts == 0 {
		return op, mrauth.ErrNoAttemptsToConfirmOperation.New() // :TODO: задокументировать возвращение operation
	}

	if err = o.checkCode(confirmingAction, confirmCode); err != nil {
		return op, err // :TODO: задокументировать возвращение operation
	}

	confirmingAction.Confirmed = true
	confirmedExpiry := confirmingAction.Expiry

	// если следующих операций нет, то всё ок!
	confirmingAction, err = op.NextNotConfirmedAction()
	if err != nil {
		if !entity.ErrOperationHasOnlyConfirmedActions.Is(err) {
			return entity.SecureOperation{}, err
		}

		op.Actions = nil
		op.Status = operationstatus.Confirmed
		op.ExpiresAt = time.Now().Add(confirmedExpiry).Round(1 * time.Second)

		return op, nil
	}

	// для нового подтверждения генерится новый токен
	op.Token, err = o.tokenGenerator.GenTokenLen(len(op.Token))
	if err != nil {
		return entity.SecureOperation{}, err
	}

	op.RemainingAttempts = confirmingAction.MaxAttempts
	op.ExpiresAt = time.Now().Add(confirmingAction.Expiry).Round(1 * time.Second)

	if confirmingAction.Method == confirmmethod.Email || confirmingAction.Method == confirmmethod.Phone {
		confirmingAction.Secret, err = o.codeGenerator.GenCodeLen(len(confirmingAction.Secret))
		if err != nil {
			return entity.SecureOperation{}, err
		}

		op.RemainingResends = confirmingAction.MaxResends
		op.ResendsAt = time.Now().Add(confirmingAction.MinResendTime).Round(1 * time.Second)

		return op, nil
	}

	// иначе это ConfirmMethodPassword или ConfirmMethodTOTP
	op.RemainingResends = 0
	op.ResendsAt = time.Time{}

	return op, nil
}

func (o *ConfirmCode) checkCode(action *dto.ConfirmAction, confirmCode string) error {
	switch action.Method {
	case confirmmethod.Password:
		if err := o.codeGenerator.CompareCodeAndHash(confirmCode, action.Secret); err == nil {
			return nil
		}
	case confirmmethod.TOTP:
		if totp.Validate(confirmCode, action.Secret) {
			return nil
		}
	default:
		if confirmCode == action.Secret {
			return nil
		}
	}

	return mrauth.ErrConfirmCodeIsIncorrect.New()
}
