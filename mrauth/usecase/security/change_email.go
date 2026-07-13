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
	// ChangeEmailProperty - создаёт операцию смены email пользователя (с проверкой
	// доступности адреса) и отправляет код её подтверждения.
	ChangeEmailProperty struct {
		txManager                   mrstorage.DBTxManager
		storageOperation            operationCreator
		emailChecker                userEmailChecker
		notifierAPI                 mrnotifier.NoteProducer
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		factoryOperationEmail       factoryOperationValue2FA
		logOperation                operationLogger
		errorWrapper                errors.Wrapper
	}

	operationCreator interface {
		Insert(ctx context.Context, row secureoperation.SecureOperation) error
	}

	userEmailChecker interface {
		CheckAvailabilityEmail(ctx context.Context, userEmail contactaddress.ContactAddress) error
	}

	factoryOperationValue2FA interface {
		Create(user2FA dto.User2FA, fieldValue string) (secureoperation.SecureOperation, error)
	}
)

// NewChangeEmailProperty - создаёт объект ChangeEmailProperty.
func NewChangeEmailProperty(
	txManager mrstorage.DBTxManager,
	storageOperation operationCreator,
	emailChecker userEmailChecker,
	notifierAPI mrnotifier.NoteProducer,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationEmail factoryOperationValue2FA,
	logOperation operationLogger,
) *ChangeEmailProperty {
	return &ChangeEmailProperty{
		txManager:                   txManager,
		storageOperation:            storageOperation,
		emailChecker:                emailChecker,
		notifierAPI:                 notifierAPI,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationEmail:       factoryOperationEmail,
		logOperation:                logOperation,
		errorWrapper:                errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - проверяет доступность нового email, создаёт операцию его смены и в той
// же транзакции отправляет пользователю код её подтверждения.
func (uc *ChangeEmailProperty) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	newEmail string,
) (secureoperation.SecureOperation, error) {
	if actor.VisitorID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	parsedEmail, err := contactaddress.ParseEmail(newEmail)
	if err != nil {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New(err)
	}

	if err := uc.emailChecker.CheckAvailabilityEmail(ctx, parsedEmail); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	user2FA, err := uc.factoryUser2FAConfirmAction.CreateByUserID(ctx, actor.VisitorID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationEmail.Create(user2FA, newEmail)
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
					"confirm.change.email",
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

	// операция смены email создана: фиксируем инициацию в журнале (запись вне транзакции)
	uc.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			op.Name, op.FirstActionMethod(), logstatus.Opened, logreason.Unspecified,
		),
	)

	return op, nil
}
