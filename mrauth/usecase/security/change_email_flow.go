package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// changeEmailFlow - общий конвейер создания операции смены email для юзкейсов, различающихся
// только цепочкой подтверждения: обычная смена адреса (ChangeEmailProperty) и смена адреса
// пользователем, утратившим доступ к текущему адресу (ChangeEmailByRecoveryProperty).
type changeEmailFlow struct {
	opener                      operationOpener
	emailChecker                userEmailChecker
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
	errorWrapper                errors.Wrapper
	noteName                    string
}

// newChangeEmailFlow - создаёт объект changeEmailFlow.
// noteName - имя шаблона уведомления с кодом подтверждения; пустое значение означает,
// что первое звено цепочки не sendable и отправлять нечего.
func newChangeEmailFlow(
	opener operationOpener,
	emailChecker userEmailChecker,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	noteName string,
) changeEmailFlow {
	return changeEmailFlow{
		opener:                      opener,
		emailChecker:                emailChecker,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		errorWrapper:                errors.NewServiceOperationFailedWrapper(),
		noteName:                    noteName,
	}
}

// execute - проверяет доступность нового email, создаёт операцию его смены выбранной
// цепочкой (createFunc) и в той же транзакции открывает её.
func (f changeEmailFlow) execute(
	ctx context.Context,
	actor dto.ActorMeta,
	newEmail contactaddress.ContactAddress,
	createFunc func(user2FA dto.User2FA, newEmail contactaddress.ContactAddress) (secureoperation.SecureOperation, error),
) (secureoperation.SecureOperation, error) {
	if actor.VisitorID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if newEmail.Value() == "" {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("newEmail is empty")
	}

	if err := f.emailChecker.CheckAvailabilityEmail(ctx, newEmail); err != nil {
		return secureoperation.SecureOperation{}, f.errorWrapper.Wrap(err)
	}

	user2FA, err := f.factoryUser2FAConfirmAction.CreateByUserID(ctx, actor.VisitorID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, f.errorWrapper.Wrap(err)
	}

	op, err := createFunc(user2FA, newEmail)
	if err != nil {
		return secureoperation.SecureOperation{}, f.errorWrapper.Wrap(err)
	}

	if err = f.opener.Open(ctx, actor, op, f.noteName, nil); err != nil {
		return secureoperation.SecureOperation{}, f.errorWrapper.Wrap(err)
	}

	return op, nil
}
