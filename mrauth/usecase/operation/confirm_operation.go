package operation

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ConfirmOperation - подтверждение защищённой операции по коду: проверка кода,
	// учёт неудачных попыток и переход к следующему действию или к статусу Confirmed.
	ConfirmOperation struct {
		txManager         mrstorage.DBTxManager
		storageOperation  operationConfirmer
		notifierAPI       mrnotifier.NoteProducer
		operationPreparer confirmOperationPreparer
		errorWrapper      errors.Wrapper
	}

	operationConfirmer interface {
		FetchOne(ctx context.Context, token string) (row secureoperation.SecureOperation, err error)
		Replace(ctx context.Context, currentToken string, row secureoperation.SecureOperation) error
		UpdateFailedAttempt(ctx context.Context, token string) (attempts int16, err error)
	}

	confirmOperationPreparer interface {
		Prepare(
			ctx context.Context,
			op secureoperation.SecureOperation,
			confirmCode string,
		) (secureoperation.SecureOperation, func(ctx context.Context) error, error)
	}
)

// NewConfirmOperation - создаёт объект ConfirmOperation.
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
		errorWrapper:      errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - подтверждает текущее действие операции по коду; при неверном коде
// уменьшает счётчик попыток, при успехе сохраняет операцию и отправляет код
// следующего действия (либо завершает операцию).
func (co *ConfirmOperation) Execute(ctx context.Context, langCode, operationToken, confirmCode string) (secureoperation.SecureOperation, error) {
	if operationToken == "" {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("operationToken is empty")
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	op, commitConfirmed, err := co.operationPreparer.Prepare(ctx, op, confirmCode)
	if err != nil {
		if errors.Is(err, secureoperation.ErrNoAttemptsToConfirmOperation) {
			return op, err // WARNING: 'op' используется с этой ошибкой
		}

		if !errors.Is(err, secureoperation.ErrConfirmCodeIsIncorrect) {
			return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
		}

		attempts, errUpdate := co.storageOperation.UpdateFailedAttempt(ctx, operationToken)
		if errUpdate != nil {
			return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(errUpdate)
		}

		// TODO: записать операцию в журнал

		op.RemainingAttempts = attempts

		if attempts > 0 {
			return op, err // WARNING: 'op' используется с этой ошибкой
		}

		// TODO: при исчерпании попыток уведомить пользователя и зафиксировать событие в журнале.
		// co.eventEmitter.Emit(
		// 	 ctx,
		// 	 "Confirm",
		// 	 "userLogin", nextConfirm.Address,
		//	 "loginType", nextConfirm.Method,
		//	 "secretCode", generateSecretCode,
		// )

		return op, secureoperation.ErrNoAttemptsToConfirmOperation.Wrap(err) // WARNING: 'op' используется с этой ошибкой
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storageOperation.Replace(ctx, operationToken, op); err != nil {
			return co.errorWrapper.Wrap(err)
		}

		// расходование аварийного кода (если он был использован) в той же транзакции
		if commitConfirmed != nil {
			if err = commitConfirmed(ctx); err != nil {
				return co.errorWrapper.Wrap(err)
			}
		}

		// TODO: записать операцию в журнал

		// если все действия подтверждены
		if op.Is(operationstatus.Confirmed) {
			// TODO: асинхронный запуск каких либо работ после подтверждения операции
			return nil
		}

		// 2fa подтверждение
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
