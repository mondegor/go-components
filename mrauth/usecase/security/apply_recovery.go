package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// ApplyRecovery - применяет подтверждённую операцию перевыпуска аварийных кодов:
	// заменяет набор кодов пользователя на новый и возвращает его (показывается один раз).
	ApplyRecovery struct {
		txManager          mrstorage.DBTxManager
		storage            recoveryCodesUpdater
		storageOperation   operationDeleter
		codeGenerator      recoveryCodesGenerator
		notifierAPI        mrauth.Notifier
		logOperation       operationLogger
		errorWrapper       errors.Wrapper
		recoveryCount      int
		recoveryCodeLength int
	}

	recoveryCodesUpdater interface {
		UpdateRecoveryCodes(ctx context.Context, userID uuid.UUID, hashed []string) error
	}
)

// NewApplyRecovery - создаёт объект ApplyRecovery.
func NewApplyRecovery(
	txManager mrstorage.DBTxManager,
	storage recoveryCodesUpdater,
	storageOperation operationDeleter,
	codeGenerator recoveryCodesGenerator,
	notifierAPI mrauth.Notifier,
	logOperation operationLogger,
	recoveryCount int,
	recoveryCodeLength int,
) *ApplyRecovery {
	recoveryCount = clampRecoveryCount(recoveryCount)

	return &ApplyRecovery{
		txManager:          txManager,
		storage:            storage,
		storageOperation:   storageOperation,
		codeGenerator:      codeGenerator,
		notifierAPI:        notifierAPI,
		logOperation:       logOperation,
		errorWrapper:       errors.NewServiceOperationFailedWrapper(),
		recoveryCount:      recoveryCount,
		recoveryCodeLength: recoveryCodeLength,
	}
}

// Execute - проверяет, что операция перевыпуска подтверждена, и в одной транзакции
// заменяет набор аварийных кодов пользователя на новый, удаляет операцию, отправляет
// уведомление и возвращает новые коды в открытом виде (показываются один раз).
func (uc *ApplyRecovery) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	operationToken string,
) (plainCodes []string, err error) {
	if actor.VisitorID == uuid.Nil {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return nil, mrauth.ErrOperationInvalid
	}

	var (
		operationType  operationtype.Enum
		actionMethod   confirmmethod.Enum
		failedLogState logState
	)

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		op, err := uc.storageOperation.FetchOneForUpdate(ctx, operationToken)
		if err != nil {
			if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
				return mrauth.ErrOperationInvalid
			}

			return uc.errorWrapper.Wrap(err)
		}

		operationType = op.Type
		actionMethod = op.FirstActionMethod()

		if actor.VisitorID != op.UserID {
			failedLogState = newLogState(logstatus.Blocked, logreason.AccessForbidden)

			return errors.ErrAccessForbidden
		}

		if op.Type != operationtype.RegenerateRecovery {
			failedLogState = newLogState(logstatus.Blocked, logreason.AccessForbidden)

			return errors.ErrAccessForbidden
		}

		if !op.Is(operationstatus.Confirmed) {
			failedLogState = newLogState(logstatus.Blocked, logreason.NotConfirmed)

			return mrauth.ErrOperationIsNotConfirmed
		}

		payload, err := unit.ParseRegenerateRecoveryPayload(op.Payload)
		if err != nil {
			return err
		}

		var hashedCodes []string

		plainCodes, hashedCodes, err = uc.codeGenerator.GenerateRecoveryCodes(uc.recoveryCount, uc.recoveryCodeLength)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storage.UpdateRecoveryCodes(ctx, op.UserID, hashedCodes); err != nil {
			// записи 2FA нет: она удалена между созданием операции и её применением
			if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
				failedLogState = newLogState(logstatus.Blocked, logreason.Auth2FAStateChanged)

				return mrauth.ErrAuth2FAIsDisabled
			}

			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return uc.notifierAPI.Send(ctx, "user.recovery_codes.changed", conv.Group{"to": payload.Email})
	})
	if err != nil {
		if failedLogState.isSet() {
			// обращение к чужой, неподходящей или ещё не подтверждённой операции
			// либо гонка с отключением 2FA: фиксируем блокировку в журнале
			uc.logOperation.Log(
				ctx,
				actor.NewOperationLog(
					operationType.String(), actionMethod, failedLogState.status, failedLogState.reason,
				),
			)
		}

		return nil, uc.errorWrapper.Wrap(err)
	}

	// операция перевыпуска recovery-кодов применена: фиксируем в журнале (запись вне транзакции)
	uc.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			operationType.String(), actionMethod, logstatus.Applied, logreason.Unspecified,
		),
	)

	return plainCodes, nil
}
