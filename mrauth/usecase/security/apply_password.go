package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ApplyPassword - применяет подтверждённую операцию смены пароля: привязывает пароль
	// как второй фактор (2FA) и выдаёт новые одноразовые аварийные коды.
	ApplyPassword struct {
		txManager        mrstorage.DBTxManager
		storage          user2faBinder
		storageOperation operationDeleter
		codeGenerator    recoveryCodesGenerator
		notifierAPI      mrnotifier.NoteProducer
		logOperation     operationLogger
		errorWrapper     errors.Wrapper
		recoveryCount    int
	}
)

// NewApplyPassword - создаёт объект ApplyPassword.
func NewApplyPassword(
	txManager mrstorage.DBTxManager,
	storage user2faBinder,
	storageOperation operationDeleter,
	codeGenerator recoveryCodesGenerator,
	notifierAPI mrnotifier.NoteProducer,
	logOperation operationLogger,
	recoveryCount int,
) *ApplyPassword {
	recoveryCount = clampRecoveryCount(recoveryCount)

	return &ApplyPassword{
		txManager:        txManager,
		storage:          storage,
		storageOperation: storageOperation,
		codeGenerator:    codeGenerator,
		notifierAPI:      notifierAPI,
		logOperation:     logOperation,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
		recoveryCount:    recoveryCount,
	}
}

// Execute - проверяет, что операция смены пароля подтверждена, и в одной транзакции
// привязывает пароль как 2FA, удаляет операцию, отправляет уведомление и возвращает
// новые аварийные коды в открытом виде (показываются один раз).
func (uc *ApplyPassword) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	operationToken string,
) (plainCodes []string, err error) {
	if actor.VisitorID == uuid.Nil {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return nil, errors.ErrRecordNotFound // TODO: возможно, стоит возвращать ошибку о некорректном параметре
	}

	var (
		operationName  string
		actionMethod   confirmmethod.Enum
		failedLogState logState
	)

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		op, err := uc.storageOperation.FetchOneForUpdate(ctx, operationToken)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		operationName = op.Name
		actionMethod = op.FirstActionMethod()

		if actor.VisitorID != op.UserID {
			failedLogState = newLogState(logstatus.Blocked, logreason.AccessForbidden)

			return errors.ErrAccessForbidden
		}

		if op.Name != unit.NameConfirmChangePassword {
			failedLogState = newLogState(logstatus.Blocked, logreason.AccessForbidden)

			return errors.ErrAccessForbidden
		}

		if !op.Is(operationstatus.Confirmed) {
			failedLogState = newLogState(logstatus.Blocked, logreason.NotConfirmed)

			return errors.New("operation is not confirmed")
		}

		payload, err := unit.ParseChangePasswordPayload(op.Payload)
		if err != nil {
			return err
		}

		var hashedCodes []string

		plainCodes, hashedCodes, err = uc.codeGenerator.GenerateRecoveryCodes(uc.recoveryCount)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storage.InsertOrUpdate(
			ctx,
			entity.Auth2FA{
				UserID:        op.UserID,
				Type:          auth2fatype.Password,
				Secret:        payload.NewPassword, // уже захеширован при создании операции
				RecoveryCodes: hashedCodes,
			},
		); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return uc.notifierAPI.Send(ctx, "user.password.changed", conv.Group{"to": payload.Email})
	})
	if err != nil {
		if failedLogState.isSet() {
			// обращение к чужой, неподходящей или ещё не подтверждённой операции:
			// фиксируем блокировку в журнале
			uc.logOperation.Log(
				ctx,
				actor.NewOperationLog(
					operationName, actionMethod, failedLogState.status, failedLogState.reason,
				),
			)
		}

		return nil, uc.errorWrapper.Wrap(err)
	}

	// операция смены пароля применена: фиксируем в журнале (запись вне транзакции)
	uc.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			operationName, actionMethod, logstatus.Applied, logreason.Unspecified,
		),
	)

	return plainCodes, nil
}
