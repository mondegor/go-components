package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// ChangeEmail - обработчик смены email пользователя: применяет операцию второго шага
	// (operationtype.ChangeEmailConfirm), когда владение новым адресом уже подтверждено.
	ChangeEmail struct {
		txManager    mrstorage.DBTxManager
		storage      userEmailChanger
		revoker      operationRevoker
		notifierAPI  mrauth.Notifier
		actorProps   actorPropsBuilder
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
	revoker operationRevoker,
	notifierAPI mrauth.Notifier,
	actorProps actorPropsBuilder,
) *ChangeEmail {
	return &ChangeEmail{
		txManager:    txManager,
		storage:      storage,
		revoker:      revoker,
		notifierAPI:  notifierAPI,
		actorProps:   actorProps,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - меняет email пользователя на новый, отзывает незавершённые операции пользователя
// и отправляет уведомления о состоявшейся смене на прежний и на новый адреса. Если новый адрес успели занять
// за время жизни операции, возвращает mrauth.ErrEmailAlreadyExists: гонка с другим пользователем
// за адрес проявляется ошибкой дубликата от UpdateEmail.
func (uc *ChangeEmail) Execute(ctx context.Context, actor dto.ActorMeta, payload []byte) error {
	if actor.UserID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	payloadDTO, err := unit.ParseChangeEmailPayload(payload)
	if err != nil {
		return err
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storage.UpdateEmail(ctx, actor.UserID, payloadDTO.NewEmail); err != nil {
			if errors.Is(err, errors.ErrInternalStorageDuplicateKeyViolation) {
				return mrauth.ErrEmailAlreadyExists
			}

			return uc.errorWrapper.Wrap(err)
		}

		// коды подтверждения и уведомления незавершённых операций привязаны к прежнему адресу,
		// поэтому операции отзываются: их нужно начать заново
		if err = uc.revoker.RevokeAll(ctx, actor, logreason.EmailChanged); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.notifierAPI.Send(
			ctx,
			"user.email.changed",
			uc.actorProps.With(actor, conv.Group{
				"to":       payloadDTO.Email,
				"oldEmail": payloadDTO.Email,
				"newEmail": payloadDTO.NewEmail,
			}),
		); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.notifierAPI.Send(
			ctx,
			"user.email.changed.new",
			uc.actorProps.With(actor, conv.Group{
				"to":       payloadDTO.NewEmail,
				"oldEmail": payloadDTO.Email,
				"newEmail": payloadDTO.NewEmail,
			}),
		); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
}
