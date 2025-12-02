package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangeProperty - comment struct.
	ChangeProperty struct {
		txManager mrstorage.DBTxManager
		// storage      mrauth.User2faStorage
		storageOperation           mrauth.SecureOperationStorage
		userChecker                mrauth.CheckUserUseCase
		notifierAPI                mrnotifier.NoticeProducer
		factoryUserConfirm2FA      mrauth.FactoryUserConfirm2FA
		factoryOperationEmail      factoryOperation
		factoryOperationPhone      factoryOperation
		factoryOperationPassword   factoryOperation
		factoryOperationTOTP       factoryOperationTOTP
		factoryOperationDisable2FA factoryOperationDisable2FA
		passwordGenerator          func() string
		errorWrapper               mrerr.UseCaseErrorWrapper
	}

	factoryOperation interface {
		Create(user2FA dto.User2FA, fieldValue string) (entity.SecureOperation, error)
	}

	factoryOperationDisable2FA interface {
		Create(user2FA dto.User2FA) (entity.SecureOperation, error)
	}

	factoryOperationTOTP factoryOperationDisable2FA
)

// NewChangeProperty - создаёт объект ChangeProperty.
func NewChangeProperty(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	userChecker mrauth.CheckUserUseCase,
	notifierAPI mrnotifier.NoticeProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationEmail factoryOperation,
	factoryOperationPhone factoryOperation,
	factoryOperationPassword factoryOperation,
	factoryOperationTOTP factoryOperationTOTP,
	factoryOperationDisable2FA factoryOperationDisable2FA,
	passwordGenerator func() string,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *ChangeProperty {
	return &ChangeProperty{
		txManager:                  txManager,
		storageOperation:           storageOperation,
		userChecker:                userChecker,
		notifierAPI:                notifierAPI,
		factoryUserConfirm2FA:      factoryUserConfirm2FA,
		factoryOperationEmail:      factoryOperationEmail,
		factoryOperationPhone:      factoryOperationPhone,
		factoryOperationPassword:   factoryOperationPassword,
		factoryOperationTOTP:       factoryOperationTOTP,
		factoryOperationDisable2FA: factoryOperationDisable2FA,
		passwordGenerator:          passwordGenerator,
		errorWrapper:               mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameRefreshToken),
	}
}

// ChangeEmail - comments method.
func (uc *ChangeProperty) ChangeEmail(ctx context.Context, userID uuid.UUID, newEmail string) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, mr.ErrUseCaseAccessForbidden.New() // TODO 401!!!!
	}

	if err := uc.userChecker.CheckAvailabilityEmail(ctx, newEmail); err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	op, err := uc.factoryOperationEmail.Create(user2FA, newEmail)
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.MaxResends > 0 {
			return uc.notifierAPI.SendNotice(ctx, "confirm.change.email", mrargs.Group{"to": confirmingAction.Address, "confirmCode": confirmingAction.Secret})
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}

// ChangePhone - comments method.
func (uc *ChangeProperty) ChangePhone(ctx context.Context, userID uuid.UUID, newPhone string) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, mr.ErrUseCaseAccessForbidden.New() // TODO 401!!!!
	}

	// TODO: проверить валидный ли телефон

	if err := uc.userChecker.CheckAvailabilityPhone(ctx, newPhone); err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	op, err := uc.factoryOperationPhone.Create(user2FA, newPhone)
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.MaxResends > 0 {
			return uc.notifierAPI.SendNotice(ctx, "confirm.change.phone", mrargs.Group{"to": confirmingAction.Address, "confirmCode": confirmingAction.Secret})
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}

// ChangePassword - comments method.
func (uc *ChangeProperty) ChangePassword(ctx context.Context, userID uuid.UUID, newPassword string) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, mr.ErrUseCaseAccessForbidden.New() // TODO 401!!!!
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	op, err := uc.factoryOperationPassword.Create(user2FA, newPassword)
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.MaxResends > 0 {
			return uc.notifierAPI.SendNotice(
				ctx,
				"confirm.change.password",
				mrargs.Group{
					"to":          confirmingAction.Address,
					"confirmCode": confirmingAction.Secret,
				},
			)
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}

// GeneratePassword - comments method.
func (uc *ChangeProperty) GeneratePassword(_ context.Context) string {
	return uc.passwordGenerator()
}

// ChangeTOTPGenerator - comments method.
func (uc *ChangeProperty) ChangeTOTPGenerator(ctx context.Context, userID uuid.UUID) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, mr.ErrUseCaseAccessForbidden.New() // TODO 401!!!!
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	op, err := uc.factoryOperationTOTP.Create(user2FA)
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.MaxResends > 0 {
			return uc.notifierAPI.SendNotice(ctx, "confirm.change.totp", mrargs.Group{"to": confirmingAction.Address, "confirmCode": confirmingAction.Secret})
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}

// Disable2FA - comments method.
func (uc *ChangeProperty) Disable2FA(ctx context.Context, userID uuid.UUID) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, mr.ErrUseCaseAccessForbidden.New() // TODO 401!!!!
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	// if user2FA.Action2FA.Method == 0 {
	// 	return entity.SecureOperation{}, errors.New("already disabled")
	// }

	op, err := uc.factoryOperationDisable2FA.Create(user2FA)
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.MaxResends > 0 {
			return uc.notifierAPI.SendNotice(ctx, "confirm.disable.2fa", mrargs.Group{"to": confirmingAction.Address, "confirmCode": confirmingAction.Secret})
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}
