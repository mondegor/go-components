package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// FactoryConfirm2FA - comment struct.
	FactoryConfirm2FA struct {
		storageUser    userStorage
		storageUser2FA user2faStorage
		factoryAction  factoryConfirmAction2FA
		errorWrapper   errors.Wrapper
	}

	userStorage interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (entity.User, error)
		FetchOneByLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (entity.User, error)
	}

	user2faStorage interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (row entity.Auth2fa, err error)
	}

	factoryConfirmAction2FA interface {
		Create(auth2fa auth2fatype.Enum, secret string) (secureoperation.ConfirmAction, error)
	}
)

// NewFactoryConfirm2FA - создаёт объект FactoryConfirm2FA.
func NewFactoryConfirm2FA(
	storageUser userStorage,
	storageUser2FA user2faStorage,
	factoryAction factoryConfirmAction2FA,
) *FactoryConfirm2FA {
	return &FactoryConfirm2FA{
		storageUser:    storageUser,
		storageUser2FA: storageUser2FA,
		factoryAction:  factoryAction,
		errorWrapper:   errors.NewServiceWrapper(),
	}
}

// CreateByUserLogin - возвращает объект для подтверждения операции пользователем с помощью 2FA.
func (sv *FactoryConfirm2FA) CreateByUserLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (dto.User2FA, error) {
	user, err := sv.storageUser.FetchOneByLogin(ctx, userLogin)
	if err != nil {
		return dto.User2FA{}, sv.errorWrapper.Wrap(err)
	}

	return sv.createUser2FA(ctx, &user)
}

// CreateByUserID - возвращает объект для подтверждения операции пользователем с помощью 2FA.
func (sv *FactoryConfirm2FA) CreateByUserID(ctx context.Context, userID uuid.UUID) (dto.User2FA, error) {
	user, err := sv.storageUser.FetchOne(ctx, userID)
	if err != nil {
		return dto.User2FA{}, sv.errorWrapper.Wrap(err)
	}

	return sv.createUser2FA(ctx, &user)
}

func (sv *FactoryConfirm2FA) createUser2FA(ctx context.Context, user *entity.User) (dto.User2FA, error) {
	// TODO: ???????????????????????????
	if user.Status != userstatus.Enabled {
		return dto.User2FA{}, errors.New("user status is not enabled")
	}

	user2fa := dto.User2FA{
		ID:    user.ID,
		Email: user.Email,
		Phone: user.Phone,
	}

	auth2fa, err := sv.storageUser2FA.FetchOne(ctx, user.ID)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRowFound) {
			return user2fa, nil
		}

		return dto.User2FA{}, sv.errorWrapper.Wrap(err)
	}

	user2fa.Action2FA, err = sv.factoryAction.Create(auth2fa.Type, auth2fa.Secret)
	if err != nil {
		return dto.User2FA{}, sv.errorWrapper.Wrap(err)
	}

	return user2fa, nil
}
