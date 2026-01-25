package secureoperation

import (
	"errors"
	"time"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

type (
	// ResendCode - comment struct.
	ResendCode struct {
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewResendCode - создаёт объект OperationFactory.
func NewResendCode(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
) *ResendCode {
	return &ResendCode{
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
	}
}

// Prepare - comments method.
func (o *ResendCode) Prepare(op entity.SecureOperation) (entity.SecureOperation, error) {
	if time.Now().After(op.ExpiresAt) {
		return entity.SecureOperation{}, mrauth.ErrOperationAlreadyExpired
	}

	// if item.Payload["audience"] == "" {
	// 	return 0, errors.New("invalid operation token")
	// }
	//
	// if item.Payload["visitor_id"] == "" {
	// 	return 0, errors.New("invalid operation token")
	// }

	if op.Status != operationstatus.Opened {
		return entity.SecureOperation{}, mrauth.ErrOperationAlreadyConfirmed // operation is not opened
	}

	confirmingAction, err := op.NextNotConfirmedAction()
	if err != nil {
		return entity.SecureOperation{}, err
	}

	if confirmingAction.MaxResends == 0 {
		return entity.SecureOperation{}, errors.New("operation not support resends")
	}

	if op.RemainingResends == 0 {
		return entity.SecureOperation{}, errors.New("operation failed resends")
	}

	if time.Now().Before(op.ResendsAt) {
		return op, mrauth.ErrSendingNewMessagesIsTemporarilyRestricted // WARNING: 'op' используется с этой ошибкой
	}

	op.Token, err = o.tokenGenerator.GenTokenLen(len(op.Token))
	if err != nil {
		return entity.SecureOperation{}, err
	}

	confirmingAction.Secret, err = o.codeGenerator.GenCodeLen(len(confirmingAction.Secret))
	if err != nil {
		return entity.SecureOperation{}, err
	}

	op.RemainingAttempts = confirmingAction.MaxAttempts
	op.RemainingResends--
	op.ResendsAt = time.Now().Add(confirmingAction.MinResendTime).Round(1 * time.Second)
	op.ExpiresAt = time.Now().Add(confirmingAction.Expiry).Round(1 * time.Second)

	return op, nil
}
