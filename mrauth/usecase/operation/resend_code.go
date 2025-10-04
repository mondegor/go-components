package operation

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ResendCode - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ResendCode struct {
		txManager         mrstorage.DBTxManager
		storageOperation  mrauth.SecureOperationStorage
		notifierAPI       mrnotifier.NoticeProducer
		operationPreparer resendOperationPreparer
		errorWrapper      mrerr.UseCaseErrorWrapper
	}

	resendOperationPreparer interface {
		Prepare(op entity.SecureOperation) (entity.SecureOperation, error)
	}
)

// NewResendCode - создаёт объект NewResendCode.
func NewResendCode(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoticeProducer,
	operationPreparer resendOperationPreparer,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *ResendCode {
	return &ResendCode{
		txManager:         txManager,
		storageOperation:  storageOperation,
		notifierAPI:       notifierAPI,
		operationPreparer: operationPreparer,
		errorWrapper:      mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameSecureOperation),
	}
}

// Perform - comments method.
func (co *ResendCode) Perform(ctx context.Context, langCode, operationToken string) (entity.SecureOperation, error) {
	if operationToken == "" {
		return entity.SecureOperation{}, mr.ErrUseCaseIncorrectInputData.New("operationToken is empty")
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	op, err = co.operationPreparer.Prepare(op)
	if err != nil {
		if mrauth.ErrSendingNewMessagesIsTemporarilyRestricted.Is(err) {
			return op, err // WARNING: 'op' используется с этой ошибкой
		}

		return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storageOperation.Update(ctx, operationToken, op); err != nil {
			return co.errorWrapper.WrapErrorFailed(err)
		}

		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return co.errorWrapper.WrapErrorFailed(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.Method != enum.ConfirmMethodEmail {
			return mr.ErrInternal.New("reason", "confirm operation method is not email")
		}

		return co.notifierAPI.SendNotice(
			ctx,
			"confirm.operation.by.email",
			mrargs.Group{
				"lang":        langCode,
				"operation":   op.Name,
				"to":          confirmingAction.Address,
				"confirmCode": confirmingAction.Secret,
			},
		)
	})
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}
