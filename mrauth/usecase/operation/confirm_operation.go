package operation

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ConfirmOperation - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ConfirmOperation struct {
		txManager         mrstorage.DBTxManager
		storageOperation  mrauth.SecureOperationStorage
		notifierAPI       mrnotifier.NoticeProducer
		operationPreparer confirmOperationPreparer
		errorWrapper      core.UseCaseErrorWrapper
	}

	confirmOperationPreparer interface {
		Prepare(op entity.SecureOperation, confirmCode string) (entity.SecureOperation, error)
	}
)

// NewConfirmOperation - создаёт объект NewConfirmOperation.
func NewConfirmOperation(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoticeProducer,
	operationPreparer confirmOperationPreparer,
) *ConfirmOperation {
	return &ConfirmOperation{
		txManager:         txManager,
		storageOperation:  storageOperation,
		notifierAPI:       notifierAPI,
		operationPreparer: operationPreparer,
		errorWrapper:      core.NewUseCaseErrorWrapper(entity.ModelNameSecureOperation),
	}
}

// Perform - возвращает строковое значение настройки с указанным идентификатором.
func (co *ConfirmOperation) Perform(ctx context.Context, langCode, operationToken, confirmCode string) (entity.SecureOperation, error) {
	if operationToken == "" {
		return entity.SecureOperation{}, mr.ErrUseCaseIncorrectInputData.New("operationToken is empty")
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	op, err = co.operationPreparer.Prepare(op, confirmCode)
	if err != nil {
		if mrauth.ErrNoAttemptsToConfirmOperation.Is(err) {
			return op, err // WARNING: 'op' используется с этой ошибкой
		}

		if !mrauth.ErrConfirmCodeIsIncorrect.Is(err) {
			return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
		}

		attempts, errUpdate := co.storageOperation.UpdateFailedAttempt(ctx, operationToken)
		if errUpdate != nil {
			return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(errUpdate)
		}

		// TODO: Add Operation log:op!

		op.RemainingAttempts = attempts

		if attempts > 0 {
			return op, err // WARNING: 'op' используется с этой ошибкой
		}

		// TODO: если тут стало 0 попыток, то отправить сообщение юзеру и зафиксировать в журнале
		// co.eventEmitter.Emit(
		// 	 ctx,
		// 	 "Confirm",
		// 	 mrargs.Group{
		// 	 	 "userLogin": nextConfirm.Address,
		//		 "loginType": nextConfirm.Method,
		//		 "secretCode": generateSecretCode,
		//	 },
		// )

		return op, mrauth.ErrNoAttemptsToConfirmOperation.Wrap(err) // WARNING: 'op' используется с этой ошибкой
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storageOperation.Update(ctx, operationToken, op); err != nil {
			return co.errorWrapper.WrapErrorFailed(err)
		}

		// если все действия подтверждены
		if op.Status == enum.OperationStatusConfirmed {
			// TODO: асинхронный запуск каких либо работ после подтверждения операции
			return nil
		}

		// 2fa подтверждение
		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return co.errorWrapper.WrapErrorFailed(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.Method == enum.ConfirmMethodEmail {
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
		}

		if confirmingAction.Method == enum.ConfirmMethodPhone {
			return mr.ErrInternal.New("reason", "ConfirmMethodPhone is not yet supported")
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}
