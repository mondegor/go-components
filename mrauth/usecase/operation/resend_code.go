package operation

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/util/operation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ResendCode - comment struct.
	ResendCode struct {
		txManager         mrstorage.DBTxManager
		storageOperation  operationResender
		notifierAPI       mrnotifier.NoteProducer
		operationPreparer resendOperationPreparer
		errorWrapper      errors.Wrapper
	}

	operationResender interface {
		FetchOne(ctx context.Context, token string) (row secureoperation.SecureOperation, err error)
		Update(ctx context.Context, currentToken string, row secureoperation.SecureOperation) error
	}

	resendOperationPreparer interface {
		Prepare(op secureoperation.SecureOperation) (secureoperation.SecureOperation, error)
	}
)

// NewResendCode - создаёт объект NewResendCode.
func NewResendCode(
	txManager mrstorage.DBTxManager,
	storageOperation operationResender,
	notifierAPI mrnotifier.NoteProducer,
	operationPreparer resendOperationPreparer,
) *ResendCode {
	return &ResendCode{
		txManager:         txManager,
		storageOperation:  storageOperation,
		notifierAPI:       notifierAPI,
		operationPreparer: operationPreparer,
		errorWrapper:      errors.NewUseCaseWrapper(),
	}
}

// Execute - comments method.
func (co *ResendCode) Execute(ctx context.Context, langCode, operationToken string) (secureoperation.SecureOperation, error) {
	if operationToken == "" {
		return secureoperation.SecureOperation{}, errors.ErrUseCaseIncorrectInputData.New("operationToken is empty")
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	op, err = co.operationPreparer.Prepare(op)
	if err != nil {
		if errors.Is(err, mrauth.ErrSendingNewMessagesIsTemporarilyRestricted) {
			return op, err // WARNING: 'op' используется с этой ошибкой
		}

		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storageOperation.Update(ctx, operationToken, op); err != nil {
			return co.errorWrapper.Wrap(err)
		}

		confirmingAction, err := operation.NextConfirmingAction(&op)
		if err != nil {
			return co.errorWrapper.Wrap(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.Method != confirmmethod.Email {
			return errors.NewInternalError("confirm operation method is not email")
		}

		return co.notifierAPI.Send(
			ctx,
			"confirm.operation.by.email",
			conv.Group{
				"lang":        langCode,
				"operation":   op.Name,
				"to":          confirmingAction.Address,
				"confirmCode": confirmingAction.Secret,
			},
		)
	})
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	return op, nil
}
