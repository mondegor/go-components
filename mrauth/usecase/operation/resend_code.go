package operation

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth/model/secureoperation"
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
		Replace(ctx context.Context, currentToken string, row secureoperation.SecureOperation) error
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
		errorWrapper:      errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - comments method.
func (co *ResendCode) Execute(ctx context.Context, langCode, operationToken string) (secureoperation.SecureOperation, error) {
	if operationToken == "" {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("operationToken is empty")
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	op, err = co.operationPreparer.Prepare(op)
	if err != nil {
		if errors.Is(err, secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted) {
			return op, err // WARNING: 'op' используется с этой ошибкой
		}

		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storageOperation.Replace(ctx, operationToken, op); err != nil {
			return co.errorWrapper.Wrap(err)
		}

		// TODO: Add Operation log:op!

		return op.NotifyByEmail(
			func(address, confirmCode string) error {
				return co.notifierAPI.Send(
					ctx,
					"confirm.operation.by.email",
					conv.Group{
						"lang":        langCode,
						"operation":   op.Name,
						"to":          address,
						"confirmCode": confirmCode,
					},
				)
			},
		)
	})
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	return op, nil
}
