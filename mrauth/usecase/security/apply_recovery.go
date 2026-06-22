package security

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ApplyRecovery - применяет подтверждённую операцию перевыпуска аварийных кодов:
	// заменяет набор кодов пользователя на новый и возвращает его (показывается один раз).
	ApplyRecovery struct {
		txManager        mrstorage.DBTxManager
		storage          recoveryCodesUpdater
		storageOperation operationDeleter
		codeGenerator    recoveryCodesGenerator
		notifierAPI      mrnotifier.NoteProducer
		errorWrapper     errors.Wrapper
		recoveryCount    int
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
	notifierAPI mrnotifier.NoteProducer,
	recoveryCount int,
) *ApplyRecovery {
	recoveryCount = clampRecoveryCount(recoveryCount)

	return &ApplyRecovery{
		txManager:        txManager,
		storage:          storage,
		storageOperation: storageOperation,
		codeGenerator:    codeGenerator,
		notifierAPI:      notifierAPI,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
		recoveryCount:    recoveryCount,
	}
}

// Execute - проверяет, что операция перевыпуска подтверждена, и в одной транзакции
// заменяет набор аварийных кодов пользователя на новый, удаляет операцию, отправляет
// уведомление и возвращает новые коды в открытом виде (показываются один раз).
func (uc *ApplyRecovery) Execute(ctx context.Context, userID uuid.UUID, operationToken string) ([]string, error) {
	if userID == uuid.Nil {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return nil, errors.ErrRecordNotFound // TODO: возможно, стоит возвращать ошибку о некорректном параметре
	}

	var plain []string

	err := uc.txManager.Do(ctx, func(ctx context.Context) error {
		op, err := uc.storageOperation.FetchOneForUpdate(ctx, operationToken)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if userID != op.UserID {
			return errors.ErrAccessForbidden
		}

		if op.Name != unit.NameConfirmRegenerateRecovery {
			return errors.ErrAccessForbidden
		}

		if !op.Is(operationstatus.Confirmed) {
			return errors.New("operation is not confirmed")
		}

		var payload dto.OperationWithUserEmail
		if err = json.Unmarshal(op.Payload, &payload); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if payload.Email == "" {
			return errors.New("operation has no staged email")
		}

		var hashed []string

		plain, hashed, err = uc.codeGenerator.GenerateRecoveryCodes(uc.recoveryCount)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storage.UpdateRecoveryCodes(ctx, op.UserID, hashed); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return uc.notifierAPI.Send(ctx, "user.recovery_codes.changed", conv.Group{"to": payload.Email})
	})
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	return plain, nil
}
