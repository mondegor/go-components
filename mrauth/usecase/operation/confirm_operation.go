package operation

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/util/operation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ConfirmOperation - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ConfirmOperation struct {
		txManager         mrstorage.DBTxManager
		storageOperation  operationConfirmer
		notifierAPI       mrnotifier.NoteProducer
		operationPreparer confirmOperationPreparer
		errorWrapper      errors.Wrapper
	}

	operationConfirmer interface {
		FetchOne(ctx context.Context, token string) (row secureoperation.SecureOperation, err error)
		Update(ctx context.Context, currentToken string, row secureoperation.SecureOperation) error
		UpdateFailedAttempt(ctx context.Context, token string) (attempts uint32, err error)
	}

	confirmOperationPreparer interface {
		Prepare(op secureoperation.SecureOperation, confirmCode string) (secureoperation.SecureOperation, error)
	}
)

// NewConfirmOperation - создаёт объект NewConfirmOperation.
func NewConfirmOperation(
	txManager mrstorage.DBTxManager,
	storageOperation operationConfirmer,
	notifierAPI mrnotifier.NoteProducer,
	operationPreparer confirmOperationPreparer,
) *ConfirmOperation {
	return &ConfirmOperation{
		txManager:         txManager,
		storageOperation:  storageOperation,
		notifierAPI:       notifierAPI,
		operationPreparer: operationPreparer,
		errorWrapper:      errors.NewUseCaseWrapper(),
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (co *ConfirmOperation) Execute(ctx context.Context, langCode, operationToken, confirmCode string) (secureoperation.SecureOperation, error) {
	if operationToken == "" {
		return secureoperation.SecureOperation{}, errors.ErrUseCaseIncorrectInputData.New("operationToken is empty")
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	op, err = co.operationPreparer.Prepare(op, confirmCode)
	if err != nil {
		if errors.Is(err, mrauth.ErrNoAttemptsToConfirmOperation) {
			return op, err // WARNING: 'op' используется с этой ошибкой
		}

		if !errors.Is(err, mrauth.ErrConfirmCodeIsIncorrect) {
			return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
		}

		attempts, errUpdate := co.storageOperation.UpdateFailedAttempt(ctx, operationToken)
		if errUpdate != nil {
			return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(errUpdate)
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
		// 	 conv.Group{
		// 	 	 "userLogin": nextConfirm.Address,
		//		 "loginType": nextConfirm.Method,
		//		 "secretCode": generateSecretCode,
		//	 },
		// )

		return op, mrauth.ErrNoAttemptsToConfirmOperation.Wrap(err) // WARNING: 'op' используется с этой ошибкой
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storageOperation.Update(ctx, operationToken, op); err != nil {
			return co.errorWrapper.Wrap(err)
		}

		// если все действия подтверждены
		if op.Status == operationstatus.Confirmed {
			// TODO: асинхронный запуск каких либо работ после подтверждения операции
			return nil
		}

		// 2fa подтверждение
		confirmingAction, err := operation.NextConfirmingAction(&op)
		if err != nil {
			return co.errorWrapper.Wrap(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.Method == confirmmethod.Email {
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
		}

		if confirmingAction.Method == confirmmethod.Phone {
			return errors.NewInternalError("ConfirmMethodPhone is not yet supported")
		}

		return nil
	})
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	return op, nil
}
