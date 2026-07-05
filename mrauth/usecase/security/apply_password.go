package security

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
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
	recoveryCount int,
) *ApplyPassword {
	recoveryCount = clampRecoveryCount(recoveryCount)

	return &ApplyPassword{
		txManager:        txManager,
		storage:          storage,
		storageOperation: storageOperation,
		codeGenerator:    codeGenerator,
		notifierAPI:      notifierAPI,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
		recoveryCount:    recoveryCount,
	}
}

// Execute - проверяет, что операция смены пароля подтверждена, и в одной транзакции
// привязывает пароль как 2FA, удаляет операцию, отправляет уведомление и возвращает
// новые аварийные коды в открытом виде (показываются один раз).
func (uc *ApplyPassword) Execute(ctx context.Context, userID uuid.UUID, operationToken string) ([]string, error) {
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

		if op.Name != unit.NameConfirmChangePassword {
			return errors.ErrAccessForbidden
		}

		if !op.Is(operationstatus.Confirmed) {
			return errors.New("operation is not confirmed")
		}

		var payload dto.ChangePasswordOperation
		if err = json.Unmarshal(op.Payload, &payload); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if payload.NewPassword == "" {
			return errors.New("operation has no staged password")
		}

		var hashed []string

		plain, hashed, err = uc.codeGenerator.GenerateRecoveryCodes(uc.recoveryCount)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storage.InsertOrUpdate(
			ctx,
			entity.Auth2FA{
				UserID:        op.UserID,
				Type:          auth2fatype.Password,
				Secret:        payload.NewPassword, // уже захеширован при создании операции
				RecoveryCodes: hashed,
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
		return nil, uc.errorWrapper.Wrap(err)
	}

	return plain, nil
}
