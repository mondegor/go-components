package operation

import (
	"context"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
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
		logOperation      operationLogger
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

	// operationLogger - best-effort продюсер записей журнала защищённых операций.
	operationLogger interface {
		Log(ctx context.Context, entry entity.SecureOperationLog)
	}
)

// NewConfirmOperation - создаёт объект ConfirmOperation.
func NewConfirmOperation(
	txManager mrstorage.DBTxManager,
	storageOperation operationConfirmer,
	notifierAPI mrnotifier.NoteProducer,
	operationPreparer confirmOperationPreparer,
	logOperation operationLogger,
) *ConfirmOperation {
	return &ConfirmOperation{
		txManager:         txManager,
		storageOperation:  storageOperation,
		notifierAPI:       notifierAPI,
		operationPreparer: operationPreparer,
		logOperation:      logOperation,
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
	actor dto.ActorMeta,
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

		// имя и метод текущего подтверждаемого действия, зафиксированные до модификации op в Prepare
		operationName string
		actionMethod  confirmmethod.Enum

		operationLogStatus logstatus.Enum
		operationLogReason logreason.Enum
	)

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		op, err = co.storageOperation.FetchOneForUpdate(ctx, operationToken)
		if err != nil {
			return co.errorWrapper.Wrap(err)
		}

		// идемпотентность: повторное подтверждение уже подтверждённой
		// операции ничего не меняет (секрет уже израсходован, действий не осталось)
		if op.Is(operationstatus.Confirmed) {
			return nil
		}

		operationName = op.Name
		actionMethod = op.FirstActionMethod()

		// владелец операции известен - он и фиксируется как посетитель
		// (поток подтверждения анонимный, в actor приходит uuid.Nil)
		actor = actor.WithVisitor(op.UserID)

		var commitConfirmed func(ctx context.Context) error

		op, commitConfirmed, err = co.operationPreparer.Prepare(ctx, op, confirmCode)
		if err != nil {
			if errors.Is(err, secureoperation.ErrNoAttemptsToConfirmOperation) {
				confirmCodeErr = err
				operationLogStatus = logstatus.Blocked
				operationLogReason = logreason.AttemptsExhausted

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

			op.RemainingAttempts = attempts

			if attempts > 0 {
				confirmCodeErr = err
				operationLogStatus = logstatus.ConfirmFailed
				operationLogReason = logreason.WrongCode

				return nil
			}

			// TODO: при исчерпании попыток уведомить пользователя.
			// co.eventEmitter.Emit(
			// 	 ctx,
			// 	 "Confirm",
			// 	 "userLogin", nextConfirm.Address,
			//	 "loginType", nextConfirm.Method,
			//	 "secretCode", generateSecretCode,
			// )

			confirmCodeErr = secureoperation.ErrNoAttemptsToConfirmOperation.Wrap(err)
			operationLogStatus = logstatus.Blocked
			operationLogReason = logreason.AttemptsExhausted

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
					operationLogStatus = logstatus.ConfirmFailed
					operationLogReason = logreason.TOTPReplay

					return err
				}

				return co.errorWrapper.Wrap(err)
			}
		}

		// если все действия подтверждены
		if op.Is(operationstatus.Confirmed) {
			operationLogStatus = logstatus.Confirmed
			operationLogReason = logreason.Unspecified

			// TODO: асинхронный запуск каких либо работ после подтверждения операции
			return nil
		}

		operationLogStatus = logstatus.ConfirmSuccess
		operationLogReason = logreason.Unspecified

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
		if operationLogReason == logreason.TOTPReplay {
			// повтор TOTP-шага / гонка 2FA: транзакция откатилась, но событие атаки фиксируем в журнале
			co.logOperation.Log(
				ctx,
				actor.NewOperationLog(
					operationName, actionMethod, operationLogStatus, operationLogReason,
				),
			)

			return secureoperation.SecureOperation{}, secureoperation.ErrConfirmCodeIsIncorrect
		}

		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	if operationName != "" {
		// транзакция зафиксирована: пишем намеченную запись журнала вне транзакции
		co.logOperation.Log(
			ctx,
			actor.NewOperationLog(
				operationName, actionMethod, operationLogStatus, operationLogReason,
			),
		)
	}

	// неверный или исчерпанный код: транзакция уже зафиксировала декремент счётчика попыток,
	// поэтому возвращается бизнес-ошибка вместе с актуальным состоянием операции
	if confirmCodeErr != nil {
		return op, confirmCodeErr // WARNING: 'op' используется с этой ошибкой
	}

	return op, nil
}
