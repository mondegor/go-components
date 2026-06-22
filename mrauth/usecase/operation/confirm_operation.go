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
		FetchOneForUpdate(ctx context.Context, token string) (row secureoperation.SecureOperation, err error)
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
// следующего действия (либо завершает операцию). Весь цикл выполняется в одной
// транзакции, что сериализует конкурентные попытки подтверждения одного токена
// и исключает «размножение» попыток.
func (co *ConfirmOperation) Execute(
	ctx context.Context,
	langCode, operationToken, confirmCode string,
) (op secureoperation.SecureOperation, err error) {
	if operationToken == "" {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("operationToken is empty")
	}

	if confirmCode == "" {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("confirmCode is empty")
	}

	var (
		// confirmCodeErr - бизнес-результат неверного или исчерпанного кода. Транзакция при этом
		// должна успешно завершиться (commit), иначе зафиксированный инкремент счётчика
		// попыток откатится; поэтому ошибка возвращается не из замыкания, а после коммита
		confirmCodeErr error

		// auth2faRace - второй фактор (аварийный код или TOTP-шаг) уже израсходован
		// конкурентным подтверждением того же пользователя; в отличие от confirmCodeErr
		// транзакция при этом откатывается, а результат отдаётся как неверный код
		auth2faRace bool
	)

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		op, err = co.storageOperation.FetchOneForUpdate(ctx, operationToken)
		if err != nil {
			return co.errorWrapper.Wrap(err)
		}

		var commitConfirmed func(ctx context.Context) error

		op, commitConfirmed, err = co.operationPreparer.Prepare(ctx, op, confirmCode)
		if err != nil {
			if errors.Is(err, secureoperation.ErrNoAttemptsToConfirmOperation) {
				confirmCodeErr = err

				return nil
			}

			if !errors.Is(err, secureoperation.ErrConfirmCodeIsIncorrect) {
				return co.errorWrapper.Wrap(err)
			}

			// далее обрабатывается ситуация связанная конкретно с ошибкой ErrConfirmCodeIsIncorrect

			attempts, errUpdate := co.storageOperation.UpdateFailedAttempt(ctx, operationToken)
			if errUpdate != nil {
				return co.errorWrapper.Wrap(errUpdate)
			}

			// TODO: записать операцию в журнал

			op.RemainingAttempts = attempts

			if attempts > 0 {
				confirmCodeErr = err

				return nil
			}

			// TODO: при исчерпании попыток уведомить пользователя и зафиксировать событие в журнале.
			// co.eventEmitter.Emit(
			// 	 ctx,
			// 	 "Confirm",
			// 	 "userLogin", nextConfirm.Address,
			//	 "loginType", nextConfirm.Method,
			//	 "secretCode", generateSecretCode,
			// )

			confirmCodeErr = secureoperation.ErrNoAttemptsToConfirmOperation.Wrap(err)

			return nil
		}

		if err = co.storageOperation.Replace(ctx, operationToken, op); err != nil {
			return co.errorWrapper.Wrap(err)
		}

		// расходование второго фактора (аварийный код или TOTP-шаг) в той же транзакции
		if commitConfirmed != nil {
			if err = commitConfirmed(ctx); err != nil {
				// гонка: второй фактор уже израсходован конкурентным подтверждением того же
				// пользователя. Откатываем транзакцию (повторное использование одного кода
				// недопустимо) и ниже отдаём это как неверный код, а не как внутреннюю ошибку
				if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
					auth2faRace = true

					return err
				}

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
		if auth2faRace {
			return secureoperation.SecureOperation{}, secureoperation.ErrConfirmCodeIsIncorrect
		}

		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	// неверный или исчерпанный код: транзакция уже зафиксировала декремент счётчика попыток,
	// поэтому возвращается бизнес-ошибка вместе с актуальным состоянием операции
	if confirmCodeErr != nil {
		return op, confirmCodeErr // WARNING: 'op' используется с этой ошибкой
	}

	return op, nil
}
