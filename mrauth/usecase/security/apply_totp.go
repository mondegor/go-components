package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
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
		txManager          mrstorage.DBTxManager
		storage            user2faBinder
		storageOperation   operationDeleter
		revoker            operationRevoker
		codeGenerator      recoveryCodesGenerator
		totpValidator      totpValidator
		notifierAPI        mrauth.Notifier
		logOperation       operationLogger
		errorWrapper       errors.Wrapper
		recoveryCount      int
		recoveryCodeLength int
	}

	user2faBinder interface {
		Insert(ctx context.Context, row entity.Auth2FA) error
	}

	recoveryCodesGenerator interface {
		GenerateRecoveryCodes(count, length int) (plain, hashed []string, err error)
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
	revoker operationRevoker,
	codeGenerator recoveryCodesGenerator,
	totpValidator totpValidator,
	notifierAPI mrauth.Notifier,
	logOperation operationLogger,
	recoveryCount int,
	recoveryCodeLength int,
) *ApplyTOTPGenerator {
	recoveryCount = clampRecoveryCount(recoveryCount)

	return &ApplyTOTPGenerator{
		txManager:          txManager,
		storage:            storage,
		storageOperation:   storageOperation,
		revoker:            revoker,
		codeGenerator:      codeGenerator,
		totpValidator:      totpValidator,
		notifierAPI:        notifierAPI,
		logOperation:       logOperation,
		errorWrapper:       errors.NewServiceOperationFailedWrapper(),
		recoveryCount:      recoveryCount,
		recoveryCodeLength: recoveryCodeLength,
	}
}

// Execute - проверяет TOTP-код, введённый пользователем, против секрета операции
// и при успехе в одной транзакции привязывает TOTP-генератор, удаляет операцию, отзывает
// все незавершённые операции пользователя, отправляет уведомление и возвращает аварийные коды в открытом виде (показываются один раз).
// Если к моменту применения 2FA уже включена (её успели включить другим способом после
// создания операции), возвращает mrauth.ErrAuth2FAMustBeDisabledFirst.
func (uc *ApplyTOTPGenerator) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	operationToken, totpCode string,
) (plainCodes []string, err error) {
	if actor.VisitorID == uuid.Nil {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if totpCode == "" {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("totpCode is empty")
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

		// TODO: проверить, что пользователь не заблокирован

		if op.Type != operationtype.ChangeTOTP {
			failedLogState = newLogState(logstatus.Blocked, logreason.AccessForbidden)

			return errors.ErrAccessForbidden
		}

		if !op.Is(operationstatus.Confirmed) {
			failedLogState = newLogState(logstatus.Blocked, logreason.NotConfirmed)

			return mrauth.ErrOperationIsNotConfirmed
		}

		payload, err := unit.ParseChangeTOTPPayload(op.Payload)
		if err != nil {
			return err
		}

		ok, timeStep, err := uc.totpValidator.ValidateCode(totpCode, payload.Secret)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if !ok {
			failedLogState = newLogState(logstatus.ConfirmFailed, logreason.WrongCode)

			return mrauth.ErrTOTPCodeIsIncorrect
		}

		var hashed []string

		plainCodes, hashed, err = uc.codeGenerator.GenerateRecoveryCodes(uc.recoveryCount, uc.recoveryCodeLength)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storage.Insert(
			ctx,
			entity.Auth2FA{
				UserID:        op.UserID,
				Type:          auth2fatype.TOTP,
				Secret:        payload.Secret,
				LastTOTPStep:  timeStep, // запоминается последний шаг успешной проверки кода, чтобы тот же код нельзя было применить повторно
				RecoveryCodes: hashed,
			},
		); err != nil {
			// 2FA включили другим способом между созданием операции и её применением:
			// активный второй фактор не перезатирается, его нужно сначала отключить
			if errors.Is(err, errors.ErrInternalStorageDuplicateKeyViolation) {
				failedLogState = newLogState(logstatus.Blocked, logreason.Auth2FAStateChanged)

				return mrauth.ErrAuth2FAMustBeDisabledFirst
			}

			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		// включение 2FA делает недействительными цепочки подтверждения всех прочих
		// незавершённых операций пользователя: они построены без второго фактора
		if err = uc.revoker.RevokeAll(ctx, actor, logreason.Auth2FAStateChanged); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return uc.notifierAPI.Send(ctx, "user.totp.changed", conv.Group{"to": payload.Email})
	})
	if err != nil {
		if failedLogState.isSet() {
			// или обращение к чужой, неподходящей или ещё не подтверждённой операции,
			// или гонка с включением 2FA другим способом: фиксируем блокировку;
			// или неверный TOTP-код: фиксируем в журнале как неудачное подтверждение;
			uc.logOperation.Log(
				ctx,
				actor.NewOperationLog(
					operationType.String(), actionMethod, failedLogState.status, failedLogState.reason,
				),
			)
		}

		return nil, uc.errorWrapper.Wrap(err)
	}

	// операция смены TOTP применена: фиксируем в журнале (запись вне транзакции)
	uc.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			operationType.String(), actionMethod, logstatus.Applied, logreason.Unspecified,
		),
	)

	return plainCodes, nil
}

// clampRecoveryCount - ограничивает число аварийных кодов диапазоном [minRecoveryCount, maxRecoveryCount]:
// оно задаёт длину bcrypt-перебора при проверке, поэтому это защита от чрезмерного значения из конфигурации хоста.
func clampRecoveryCount(count int) int {
	if count < minRecoveryCount {
		return minRecoveryCount
	}

	if count > maxRecoveryCount {
		return maxRecoveryCount
	}

	return count
}
