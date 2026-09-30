package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// ChangeEmail - обработчик смены email пользователя: применяет операцию второго шага
	// (operationtype.ChangeEmailConfirm), когда владение новым адресом уже подтверждено.
	ChangeEmail struct {
		txManager    mrstorage.DBTxManager
		storage      userEmailChanger
		notifierAPI  mrauth.Notifier
		errorWrapper errors.Wrapper
	}

	userEmailChanger interface {
		UpdateEmail(ctx context.Context, userID uuid.UUID, value string) error
	}
)

// NewChangeEmail - создаёт объект ChangeEmail.
func NewChangeEmail(
	txManager mrstorage.DBTxManager,
	storage userEmailChanger,
	notifierAPI mrauth.Notifier,
) *ChangeEmail {
	return &ChangeEmail{
		txManager:    txManager,
		storage:      storage,
		notifierAPI:  notifierAPI,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - меняет email пользователя на новый и отправляет на прежний адрес уведомление
// о состоявшейся смене. Если новый адрес успели занять за время жизни операции, возвращает
// mrauth.ErrEmailAlreadyExists: гонка с другим пользователем за адрес проявляется ошибкой
// дубликата от UpdateEmail.
func (uc *ChangeEmail) Execute(ctx context.Context, actor dto.ActorMeta, payload []byte) error {
	if actor.VisitorID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	payloadDTO, err := unit.ParseChangeEmailPayload(payload)
	if err != nil {
		return err
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.UpdateEmail(ctx, actor.VisitorID, payloadDTO.NewEmail); err != nil {
			if errors.Is(err, errors.ErrInternalStorageDuplicateKeyViolation) {
				return mrauth.ErrEmailAlreadyExists
			}

			return uc.errorWrapper.Wrap(err)
		}

		if err := uc.notifierAPI.Send(ctx, "user.email.changed", conv.Group{"to": payloadDTO.Email}); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
}
