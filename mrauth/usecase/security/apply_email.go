package security

import (
	"context"
	"time"

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
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// ApplyEmail - применяет подтверждённую операцию первого шага смены емаила (владение
	// аккаунтом доказано): емаил пока не меняется, вместо этого открывается операция второго
	// шага - подтверждения владения новым адресом, а на прежний адрес уходит уведомление
	// о запросе смены.
	ApplyEmail struct {
		txManager        mrstorage.DBTxManager
		storageOperation operationDeleter
		emailChecker     userEmailChecker
		factoryConfirm   changeEmailCreator
		opener           operationOpener
		notifierAPI      mrauth.Notifier
		logOperation     operationLogger
		errorWrapper     errors.Wrapper
	}

	// changeEmailCreator - фабрика операции второго шага смены емаила.
	changeEmailCreator interface {
		Create(userID uuid.UUID, in dto.ChangeEmailOperation) (secureoperation.SecureOperation, error)
	}
)

// NewApplyEmail - создаёт объект ApplyEmail.
func NewApplyEmail(
	txManager mrstorage.DBTxManager,
	storageOperation operationDeleter,
	emailChecker userEmailChecker,
	factoryConfirm changeEmailCreator,
	opener operationOpener,
	notifierAPI mrauth.Notifier,
	logOperation operationLogger,
) *ApplyEmail {
	return &ApplyEmail{
		txManager:        txManager,
		storageOperation: storageOperation,
		emailChecker:     emailChecker,
		factoryConfirm:   factoryConfirm,
		opener:           opener,
		notifierAPI:      notifierAPI,
		logOperation:     logOperation,
		errorWrapper:     errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - проверяет, что операция первого шага смены емаила подтверждена и принадлежит
// пользователю, и в одной транзакции удаляет её, открывает операцию второго шага (код уходит
// на новый адрес; прежняя операция второго шага, если была, вытесняется) и отправляет на
// прежний адрес уведомление о запросе смены. Возвращает операцию второго шага.
// Если новый адрес успели занять, пока шло подтверждение, возвращает mrauth.ErrEmailAlreadyExists.
// Срок действия новой операции в уведомлении выводится в часовом поясе userLocation
// (nil - UTC), тем же, в котором пользователю отдаются даты в ответах.
func (uc *ApplyEmail) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	userLocation *time.Location,
	operationToken string,
) (secureoperation.SecureOperation, error) {
	if actor.VisitorID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return secureoperation.SecureOperation{}, mrauth.ErrOperationInvalid
	}

	var (
		operationName  string
		actionMethod   confirmmethod.Enum
		failedLogState logState
		confirmOp      secureoperation.SecureOperation
	)

	err := uc.txManager.Do(ctx, func(ctx context.Context) error {
		op, err := uc.storageOperation.FetchOneForUpdate(ctx, operationToken)
		if err != nil {
			if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
				return mrauth.ErrOperationInvalid
			}

			return uc.errorWrapper.Wrap(err)
		}

		operationName = op.Name
		actionMethod = op.FirstActionMethod()

		if actor.VisitorID != op.UserID {
			failedLogState = newLogState(logstatus.Blocked, logreason.AccessForbidden)

			return errors.ErrAccessForbidden
		}

		if op.Name != unit.NameConfirmChangeEmailRequest {
			failedLogState = newLogState(logstatus.Blocked, logreason.AccessForbidden)

			return errors.ErrAccessForbidden
		}

		if !op.Is(operationstatus.Confirmed) {
			failedLogState = newLogState(logstatus.Blocked, logreason.NotConfirmed)

			return mrauth.ErrOperationIsNotConfirmed
		}

		payload, err := unit.ParseChangeEmailPayload(op.Payload)
		if err != nil {
			return err
		}

		// адрес был свободен при создании операции, но его могли занять, пока шло подтверждение
		if err = uc.emailChecker.CheckAvailabilityEmail(ctx, contactaddress.NewEmail(payload.NewEmail)); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		confirmOp, err = uc.factoryConfirm.Create(op.UserID, payload)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		// транзакция вложенная: Open выполняется в текущей, поэтому операция второго шага,
		// код на новый адрес и уведомление на прежний фиксируются вместе с удалением первой
		if err = uc.opener.Open(ctx, actor, confirmOp, "confirm.change.email", nil); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return uc.notifierAPI.Send(
			ctx,
			"user.email.change.requested",
			conv.Group{
				"to":        payload.Email,
				"newEmail":  payload.NewEmail,
				"expiresAt": formatNoticeTime(confirmOp.ExpiresAt, userLocation),
			},
		)
	})
	if err != nil {
		if failedLogState.isSet() {
			// обращение к чужой, неподходящей или ещё не подтверждённой операции:
			// фиксируем блокировку в журнале даже при откате транзакции
			uc.logOperation.Log(
				ctx,
				actor.NewOperationLog(
					operationName, actionMethod, failedLogState.status, failedLogState.reason,
				),
			)
		}

		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	// первый шаг смены емаила применён: фиксируем в журнале (запись вне транзакции)
	uc.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			operationName, actionMethod, logstatus.Applied, logreason.Unspecified,
		),
	)

	return confirmOp, nil
}

// formatNoticeTime - форматирует момент для текста уведомления: дата и время в часовом поясе
// loc (nil - UTC) с его названием, чтобы читатель письма не гадал, в каком поясе указан срок.
func formatNoticeTime(tm time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}

	return tm.In(loc).Format("2006-01-02 15:04") + " (" + loc.String() + ")"
}
