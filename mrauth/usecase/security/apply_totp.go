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

const (
	minRecoveryCount = 2
	maxRecoveryCount = 32
)

type (
	// ApplyTOTPGenerator - проверяет введённый TOTP-код против секрета, сохранённого
	// в payload подтверждённой операции, и при успехе привязывает TOTP-генератор
	// пользователю, выдавая одноразовые аварийные коды.
	ApplyTOTPGenerator struct {
		txManager        mrstorage.DBTxManager
		storage          user2faBinder
		storageOperation operationDeleter
		codeGenerator    recoveryCodesGenerator
		totpValidator    totpValidator
		notifierAPI      mrnotifier.NoteProducer
		errorWrapper     errors.Wrapper
		recoveryCount    int
	}

	user2faBinder interface {
		InsertOrUpdate(ctx context.Context, row entity.Auth2fa) error
	}

	recoveryCodesGenerator interface {
		GenerateRecoveryCodes(count int) (plain, hashed []string, err error)
	}

	totpValidator interface {
		ValidateCode(code, secret string) (ok bool, timeStep int64, err error)
	}
)

// NewApplyTOTPGenerator - создаёт объект ApplyTOTPGenerator.
func NewApplyTOTPGenerator(
	txManager mrstorage.DBTxManager,
	storage user2faBinder,
	storageOperation operationDeleter,
	codeGenerator recoveryCodesGenerator,
	totpValidator totpValidator,
	notifierAPI mrnotifier.NoteProducer,
	recoveryCount int,
) *ApplyTOTPGenerator {
	// ограничивается число аварийных кодов: оно задаёт длину bcrypt-перебора при проверке,
	// поэтому это является защитой от чрезмерного значения из конфигурации хоста.
	if recoveryCount < minRecoveryCount {
		recoveryCount = minRecoveryCount
	}

	if recoveryCount > maxRecoveryCount {
		recoveryCount = maxRecoveryCount
	}

	return &ApplyTOTPGenerator{
		txManager:        txManager,
		storage:          storage,
		storageOperation: storageOperation,
		codeGenerator:    codeGenerator,
		totpValidator:    totpValidator,
		notifierAPI:      notifierAPI,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
		recoveryCount:    recoveryCount,
	}
}

// Execute - проверяет TOTP-код, введённый пользователем, против секрета операции
// и при успехе в одной транзакции привязывает TOTP-генератор, удаляет операцию,
// отправляет уведомление и возвращает аварийные коды в открытом виде (показываются один раз).
func (uc *ApplyTOTPGenerator) Execute(ctx context.Context, userID uuid.UUID, operationToken, totpCode string) ([]string, error) {
	if userID == uuid.Nil {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if totpCode == "" {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("totpCode is empty")
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

		// TODO: проверить, что пользователь не заблокирован

		if op.Name != unit.NameConfirmChangeTOTP {
			return errors.ErrAccessForbidden
		}

		if !op.Is(operationstatus.Confirmed) {
			return errors.New("operation is not confirmed")
		}

		var payload dto.ChangeTotpOperation
		if err = json.Unmarshal(op.Payload, &payload); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if payload.Secret == "" {
			return errors.New("operation has no staged secret")
		}

		ok, timeStep, err := uc.totpValidator.ValidateCode(totpCode, payload.Secret)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if !ok {
			return errors.ErrIncorrectInputData.New("invalid totp code")
		}

		var hashed []string

		plain, hashed, err = uc.codeGenerator.GenerateRecoveryCodes(uc.recoveryCount)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storage.InsertOrUpdate(
			ctx,
			entity.Auth2fa{
				UserID:        op.UserID,
				Type:          auth2fatype.TOTP,
				Secret:        payload.Secret,
				LastTOTPStep:  timeStep, // запоминается последний шаг успешной проверки кода, чтобы тот же код нельзя было применить повторно
				RecoveryCodes: hashed,
			},
		); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return uc.notifierAPI.Send(ctx, "user.totp.changed", conv.Group{"to": payload.Email})
	})
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	return plain, nil
}
