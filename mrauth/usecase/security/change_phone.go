package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangePhoneProperty - создаёт операцию смены телефона пользователя (с проверкой
	// доступности номера) и отправляет код её подтверждения.
	ChangePhoneProperty struct {
		txManager                   mrstorage.DBTxManager
		storageOperation            operationCreator
		phoneChecker                userPhoneChecker
		notifierAPI                 mrnotifier.NoteProducer
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		factoryOperationPhone       factoryOperationValue2FA
		logOperation                operationLogger
		errorWrapper                errors.Wrapper
	}

	userPhoneChecker interface {
		CheckAvailabilityPhone(ctx context.Context, userPhone contactaddress.ContactAddress) error
	}
)

// NewChangePhoneProperty - создаёт объект ChangePhoneProperty.
func NewChangePhoneProperty(
	txManager mrstorage.DBTxManager,
	storageOperation operationCreator,
	phoneChecker userPhoneChecker,
	notifierAPI mrnotifier.NoteProducer,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationPhone factoryOperationValue2FA,
	logOperation operationLogger,
) *ChangePhoneProperty {
	return &ChangePhoneProperty{
		txManager:                   txManager,
		storageOperation:            storageOperation,
		phoneChecker:                phoneChecker,
		notifierAPI:                 notifierAPI,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationPhone:       factoryOperationPhone,
		logOperation:                logOperation,
		errorWrapper:                errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - проверяет доступность нового телефона, создаёт операцию его смены и в той
// же транзакции отправляет пользователю код её подтверждения.
func (uc *ChangePhoneProperty) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	newPhone string,
) (secureoperation.SecureOperation, error) {
	if actor.VisitorID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	parsedPhone, err := contactaddress.ParsePhone(newPhone)
	if err != nil {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New(err)
	}

	if err := uc.phoneChecker.CheckAvailabilityPhone(ctx, parsedPhone); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	user2FA, err := uc.factoryUser2FAConfirmAction.CreateByUserID(ctx, actor.VisitorID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationPhone.Create(user2FA, newPhone)
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return op.NotifyByEmail(
			func(address, confirmCode string) error {
				return uc.notifierAPI.Send(
					ctx,
					"confirm.change.phone",
					conv.Group{
						"to":          address,
						"confirmCode": confirmCode,
					},
				)
			},
		)
	})
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	// операция смены телефона создана: фиксируем инициацию в журнале (запись вне транзакции)
	uc.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			op.Name, op.FirstActionMethod(), logstatus.Opened, logreason.Unspecified,
		),
	)

	return op, nil
}
