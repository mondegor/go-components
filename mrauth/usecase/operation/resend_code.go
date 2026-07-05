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
	// ResendCode - повторная отправка кода подтверждения защищённой операции пользователя.
	ResendCode struct {
		txManager         mrstorage.DBTxManager
		storageOperation  operationResender
		notifierAPI       mrnotifier.NoteProducer
		operationPreparer resendOperationPreparer
		errorWrapper      errors.Wrapper
	}

	operationResender interface {
		FetchOneForUpdate(ctx context.Context, token string) (row secureoperation.SecureOperation, err error)
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

// Execute - повторно отправляет код подтверждения текущего действия операции.
// Выполняется в одной транзакции, что исключает гонку повторной отправки с подтверждением того же токена.
func (co *ResendCode) Execute(ctx context.Context, langCode, operationToken string) (op secureoperation.SecureOperation, err error) {
	if operationToken == "" {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("operationToken is empty")
	}

	// resendCodeErr - бизнес-результат временной невозможности повторной отправки кода
	var resendCodeErr error

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		op, err = co.storageOperation.FetchOneForUpdate(ctx, operationToken)
		if err != nil {
			return co.errorWrapper.Wrap(err)
		}

		op, err = co.operationPreparer.Prepare(op)
		if err != nil {
			if errors.Is(err, secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted) {
				resendCodeErr = err

				return nil
			}

			return co.errorWrapper.Wrap(err)
		}

		if err = co.storageOperation.Replace(ctx, operationToken, op); err != nil {
			return co.errorWrapper.Wrap(err)
		}

		// TODO: записать операцию в журнал

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

	// возвращается бизнес-ошибка вместе с актуальным состоянием операции
	if resendCodeErr != nil {
		return op, resendCodeErr // WARNING: 'op' используется с этой ошибкой
	}

	return op, nil
}
