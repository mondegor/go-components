package operation

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// RevokeOperation - usecase отзыва (удаления) защищённой операции.
	RevokeOperation struct {
		txManager          mrstorage.DBTxManager
		storageOperation   operationRevoker
		storageSecurityLog securityLogStorage
		logOperation       operationLogger
		errorWrapper       errors.Wrapper
	}

	operationRevoker interface {
		FetchOne(ctx context.Context, token string) (secureoperation.SecureOperation, error)
		Delete(ctx context.Context, token string) error
	}

	// securityLogStorage - хранилище журнала безопасности пользователя; запись выполняется
	// в транзакции вызывающего, поэтому событие фиксируется вместе с самим отзывом.
	securityLogStorage interface {
		Insert(ctx context.Context, row entity.SecurityLogEvent) error
	}
)

// NewRevokeOperation - создаёт объект RevokeOperation.
func NewRevokeOperation(
	txManager mrstorage.DBTxManager,
	storageOperation operationRevoker,
	storageSecurityLog securityLogStorage,
	logOperation operationLogger,
) *RevokeOperation {
	return &RevokeOperation{
		txManager:          txManager,
		storageOperation:   storageOperation,
		storageSecurityLog: storageSecurityLog,
		logOperation:       logOperation,
		errorWrapper:       errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - проверяет, что операция принадлежит вызывающему, и отзывает (удаляет) её по токену.
// Операция читается перед удалением, чтобы сверить владельца и чтобы в журнал попало,
// что именно было отозвано. Отзыв смены емаила в той же транзакции записывается
// в журнал безопасности пользователя.
func (co *RevokeOperation) Execute(ctx context.Context, actor dto.ActorMeta, operationToken string) error {
	// поток отзыва только для залогиненных, поэтому анонимный вызывающий - ошибка проводки
	if actor.UserID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return mrauth.ErrOperationInvalid
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return mrauth.ErrOperationInvalid
		}

		return co.errorWrapper.Wrap(err)
	}

	// отозвать можно только собственную операцию: владение её токеном доступа не даёт
	if actor.UserID != op.UserID {
		// обращение к чужой операции: фиксируем блокировку в журнале
		co.logOperation.Log(
			ctx,
			actor.NewOperationLog(
				op.Type.String(), op.FirstActionMethod(), logstatus.Blocked, logreason.AccessForbidden,
			),
		)

		return errors.ErrAccessForbidden
	}

	// в журнал безопасности попадает только отзыв смены емаила - единственной операции,
	// ожидающей пользователя (см. unit.PendingOperationTypes); отзыв прочих - это отказ
	// от незавершённого step-up, событием безопасности он не является. Запрос смены,
	// закрытый новым запросом, включением или снятием 2FA либо истечением срока,
	// в журнал не пишется: кроме истечения срока, за каждым из них в журнале следует
	// закрывшее его событие
	var securityEvent *entity.SecurityLogEvent

	if op.Type == operationtype.ChangeEmailConfirm {
		payload, err := unit.ParseChangeEmailPayload(op.Payload)
		if err != nil {
			return co.errorWrapper.Wrap(err)
		}

		event := actor.NewSecurityEvent(securityevent.EmailChangeRevoked, &entity.SecurityLogExtra{NewValue: payload.NewEmail})
		securityEvent = &event
	}

	// операция потребляется по тому же предъявленному токену и без блокировки, взятой выше:
	// конкурентный отзыв мог удалить строку между выборкой и удалением - это тот же
	// «токен больше не действует», а не нарушение инварианта
	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err := co.storageOperation.Delete(ctx, operationToken); err != nil {
			if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
				return mrauth.ErrOperationInvalid
			}

			return err
		}

		if securityEvent == nil {
			return nil
		}

		return co.storageSecurityLog.Insert(ctx, *securityEvent)
	})
	if err != nil {
		return co.errorWrapper.Wrap(err)
	}

	// операция отозвана: фиксируем в журнале
	co.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			op.Type.String(), op.FirstActionMethod(), logstatus.Revoked, logreason.Unspecified,
		),
	)

	return nil
}
