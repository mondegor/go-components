package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
)

type (
	// FactoryConfirm2FA - comment struct.
	FactoryConfirm2FA struct {
		storageUser    mrauth.UserStorage
		storageUser2FA mrauth.User2faStorage
		factoryAction  factoryConfirmAction2FA
		errorWrapper   errors.Wrapper
	}

	factoryConfirmAction2FA interface {
		Create(auth2fa auth2fatype.Enum, secret string) (dto.ConfirmAction, error)
	}
)

// NewFactoryConfirm2FA - создаёт объект FactoryConfirm2FA.
func NewFactoryConfirm2FA(
	storageUser mrauth.UserStorage,
	storageUser2FA mrauth.User2faStorage,
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
func (re *FactoryConfirm2FA) CreateByUserLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (dto.User2FA, error) {
	user, err := re.storageUser.FetchOneByLogin(ctx, userLogin)
	if err != nil {
		return dto.User2FA{}, re.errorWrapper.Wrap(err)
	}

	return re.createUser2FA(ctx, &user)
}

// CreateByUserID - возвращает объект для подтверждения операции пользователем с помощью 2FA.
func (re *FactoryConfirm2FA) CreateByUserID(ctx context.Context, userID uuid.UUID) (dto.User2FA, error) {
	user, err := re.storageUser.FetchOne(ctx, userID)
	if err != nil {
		return dto.User2FA{}, re.errorWrapper.Wrap(err)
	}

	return re.createUser2FA(ctx, &user)
}

func (re *FactoryConfirm2FA) createUser2FA(ctx context.Context, user *entity.User) (dto.User2FA, error) {
	// TODO: ???????????????????????????
	if user.Status != userstatus.Enabled {
		return dto.User2FA{}, errors.New("user status is not enabled")
	}

	user2fa := dto.User2FA{
		ID:    user.ID,
		Email: user.Email,
		Phone: user.Phone,
	}

	auth2fa, err := re.storageUser2FA.FetchOne(ctx, user.ID)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRowFound) {
			return user2fa, nil
		}

		return dto.User2FA{}, re.errorWrapper.Wrap(err)
	}

	user2fa.Action2FA, err = re.factoryAction.Create(auth2fa.Type, auth2fa.Secret)
	if err != nil {
		return dto.User2FA{}, re.errorWrapper.Wrap(err)
	}

	return user2fa, nil
}
