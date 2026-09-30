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

type (
	// ApplyPassword - применяет подтверждённую операцию смены пароля: привязывает пароль
	// как второй фактор (2FA) и выдаёт новые одноразовые аварийные коды.
	ApplyPassword struct {
		txManager          mrstorage.DBTxManager
		storage            user2faBinder
		storageOperation   operationDeleter
		revoker            operationRevoker
		codeGenerator      recoveryCodesGenerator
		notifierAPI        mrauth.Notifier
		actorProps         actorPropsBuilder
		logOperation       operationLogger
		errorWrapper       errors.Wrapper
		recoveryCount      int
		recoveryCodeLength int
	}

	// operationRevoker - отзывает все незавершённые операции пользователя (их цепочки
	// подтверждения стали недействительными, например после смены состояния 2FA).
	operationRevoker interface {
		RevokeAll(ctx context.Context, actor dto.ActorMeta, reason logreason.Enum) error
	}
)

// NewApplyPassword - создаёт объект ApplyPassword.
func NewApplyPassword(
	txManager mrstorage.DBTxManager,
	storage user2faBinder,
	storageOperation operationDeleter,
	revoker operationRevoker,
	codeGenerator recoveryCodesGenerator,
	notifierAPI mrauth.Notifier,
	actorProps actorPropsBuilder,
	logOperation operationLogger,
	recoveryCount int,
	recoveryCodeLength int,
) *ApplyPassword {
	recoveryCount = clampRecoveryCount(recoveryCount)

	return &ApplyPassword{
		txManager:          txManager,
		storage:            storage,
		storageOperation:   storageOperation,
		revoker:            revoker,
		codeGenerator:      codeGenerator,
		notifierAPI:        notifierAPI,
		actorProps:         actorProps,
		logOperation:       logOperation,
		errorWrapper:       errors.NewServiceOperationFailedWrapper(),
		recoveryCount:      recoveryCount,
		recoveryCodeLength: recoveryCodeLength,
	}
}

// Execute - проверяет, что операция смены пароля подтверждена, и в одной транзакции
// привязывает пароль как 2FA, удаляет операцию, отзывает незавершённые операции пользователя, отправляет
// уведомление о включении 2FA и возвращает новые аварийные коды в открытом виде (показываются один раз).
// Если к моменту применения 2FA уже включена (её успели включить другим способом после
// создания операции), возвращает mrauth.ErrAuth2FAMustBeDisabledFirst.
func (uc *ApplyPassword) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	operationToken string,
) (plainCodes []string, err error) {
	if actor.UserID == uuid.Nil {
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

		if actor.UserID != op.UserID {
			failedLogState = newLogState(logstatus.Blocked, logreason.AccessForbidden)

			return errors.ErrAccessForbidden
		}

		if op.Type != operationtype.ChangePassword {
			failedLogState = newLogState(logstatus.Blocked, logreason.AccessForbidden)

			return errors.ErrAccessForbidden
		}

		if !op.Is(operationstatus.Confirmed) {
			failedLogState = newLogState(logstatus.Blocked, logreason.NotConfirmed)

			return mrauth.ErrOperationIsNotConfirmed
		}

		payload, err := unit.ParseChangePasswordPayload(op.Payload)
		if err != nil {
			return err
		}

		var hashedCodes []string

		plainCodes, hashedCodes, err = uc.codeGenerator.GenerateRecoveryCodes(uc.recoveryCount, uc.recoveryCodeLength)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storage.Insert(
			ctx,
			entity.Auth2FA{
				UserID:        op.UserID,
				Type:          auth2fatype.Password,
				Secret:        payload.NewPassword, // уже захеширован при создании операции
				RecoveryCodes: hashedCodes,
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

		return uc.notifierAPI.Send(
			ctx,
			"user.2fa.enabled",
			uc.actorProps.With(actor, conv.Group{
				"to":     payload.Email,
				"factor": auth2fatype.Password.String(),
			}),
		)
	})
	if err != nil {
		if failedLogState.isSet() {
			// обращение к чужой, неподходящей или ещё не подтверждённой операции
			// либо гонка с включением 2FA другим способом: фиксируем блокировку в журнале
			uc.logOperation.Log(
				ctx,
				actor.NewOperationLog(
					operationType.String(), actionMethod, failedLogState.status, failedLogState.reason,
				),
			)
		}

		return nil, uc.errorWrapper.Wrap(err)
	}

	// операция смены пароля применена: фиксируем в журнале (запись вне транзакции)
	uc.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			operationType.String(), actionMethod, logstatus.Applied, logreason.Unspecified,
		),
	)

	return plainCodes, nil
}
