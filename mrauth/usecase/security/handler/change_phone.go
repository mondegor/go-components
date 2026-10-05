package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/casttype"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// ChangePhone - обработчик смены телефона пользователя.
	ChangePhone struct {
		txManager          mrstorage.DBTxManager
		storage            userPhoneChanger
		notifierAPI        mrauth.Notifier
		actorProps         actorPropsBuilder
		storageSecurityLog securityLogStorage
		errorWrapper       errors.Wrapper
	}

	userPhoneChanger interface {
		UpdatePhone(ctx context.Context, userID uuid.UUID, value uint64) error
	}
)

// NewChangePhone - создаёт объект ChangePhone.
func NewChangePhone(
	txManager mrstorage.DBTxManager,
	storage userPhoneChanger,
	notifierAPI mrauth.Notifier,
	actorProps actorPropsBuilder,
	storageSecurityLog securityLogStorage,
) *ChangePhone {
	return &ChangePhone{
		txManager:          txManager,
		storage:            storage,
		notifierAPI:        notifierAPI,
		actorProps:         actorProps,
		storageSecurityLog: storageSecurityLog,
		errorWrapper:       errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - применяет подтверждённую операцию смены телефона пользователя, записывает смену
// в журнал безопасности с прежним и новым номером и отправляет уведомление о ней с контекстом клиента.
func (uc *ChangePhone) Execute(ctx context.Context, actor dto.ActorMeta, payload []byte) error {
	if actor.UserID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	payloadDTO, err := unit.ParseChangePhonePayload(payload)
	if err != nil {
		return err
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storage.UpdatePhone(ctx, actor.UserID, payloadDTO.NewPhone); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.storageSecurityLog.Insert(
			ctx,
			actor.NewSecurityEvent(
				securityevent.PhoneChanged,
				&entity.SecurityLogExtra{
					OldValue: casttype.UintToPhone(payloadDTO.Phone),
					NewValue: casttype.UintToPhone(payloadDTO.NewPhone),
				},
			),
		); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.notifierAPI.Send(ctx, "user.phone.changed", uc.actorProps.With(actor, conv.Group{"to": payloadDTO.Email})); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
}
