package check

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
)

type (
	// UserLogin - сервис для проверки доступности емаила или телефона пользователя.
	// Сервис не проверяет существование самого realm.
	UserLogin struct {
		storageCheckUser checkUserStorage
		storageUserRealm userRealmStorage
		errorWrapper     errors.Wrapper
	}

	checkUserStorage interface {
		UserIDByEmail(ctx context.Context, userEmail string) (rowID uuid.UUID, err error)
		UserIDByPhone(ctx context.Context, userPhone uint64) (rowID uuid.UUID, err error)
	}

	userRealmStorage interface {
		FetchOne(ctx context.Context, userID uuid.UUID, realm string) (row entity.UserRealm, err error)
	}
)

// NewUserLogin - создаёт объект UserLogin.
func NewUserLogin(
	storageCheckUser checkUserStorage,
	storageUserRealm userRealmStorage,
) *UserLogin {
	return &UserLogin{
		storageCheckUser: storageCheckUser,
		storageUserRealm: storageUserRealm,
		errorWrapper:     errors.NewServiceOperationFailedWrapper(),
	}
}

// CheckAvailability - проверяет, что указанный логин не существует ни в одном realm.
func (sv *UserLogin) CheckAvailability(ctx context.Context, userLogin contactaddress.ContactAddress) error {
	return sv.CheckAvailabilityRealm(ctx, "", userLogin)
}

// CheckAvailabilityEmail - проверяет, что указанный email не существует ни в одном realm.
// Если email существует, то вернётся mrauth.ErrEmailAlreadyExists.
func (sv *UserLogin) CheckAvailabilityEmail(ctx context.Context, userEmail contactaddress.ContactAddress) error {
	return sv.checkAvailabilityRealmEmail(ctx, "", userEmail.Value())
}

// CheckAvailabilityRealm - проверяет, что указанный логин не существует в указанном realm.
func (sv *UserLogin) CheckAvailabilityRealm(ctx context.Context, realm string, userLogin contactaddress.ContactAddress) error {
	if userLogin.Is(addresstype.Email) {
		return sv.checkAvailabilityRealmEmail(ctx, realm, userLogin.Value())
	}

	if userLogin.Is(addresstype.Phone) {
		return sv.checkAvailabilityRealmPhone(ctx, realm, userLogin.DigitValue())
	}

	return contactaddress.ErrAddressIsInvalid
}

func (sv *UserLogin) checkAvailabilityRealmEmail(ctx context.Context, realm, userEmail string) error {
	userID, err := sv.storageCheckUser.UserIDByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return nil
		}

		return sv.errorWrapper.Wrap(err)
	}

	if realm == "" {
		return mrauth.ErrEmailAlreadyExists
	}

	return sv.checkUserRealm(ctx, userID, realm, mrauth.ErrEmailAlreadyExists)
}

// CheckAvailabilityPhone - проверяет, что указанный телефон не существует ни в одном realm.
// Если телефон существует, то вернётся mrauth.ErrPhoneAlreadyExists.
func (sv *UserLogin) CheckAvailabilityPhone(ctx context.Context, userPhone contactaddress.ContactAddress) error {
	return sv.checkAvailabilityRealmPhone(ctx, "", userPhone.DigitValue())
}

func (sv *UserLogin) checkAvailabilityRealmPhone(ctx context.Context, realm string, userPhone uint64) error {
	userID, err := sv.storageCheckUser.UserIDByPhone(ctx, userPhone)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return nil
		}

		return sv.errorWrapper.Wrap(err)
	}

	if realm == "" {
		return mrauth.ErrPhoneAlreadyExists
	}

	return sv.checkUserRealm(ctx, userID, realm, mrauth.ErrPhoneAlreadyExists)
}

func (sv *UserLogin) checkUserRealm(ctx context.Context, userID uuid.UUID, realm string, errIfExists error) error {
	if _, err := sv.storageUserRealm.FetchOne(ctx, userID, realm); err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return nil
		}

		return sv.errorWrapper.Wrap(err)
	}

	return errIfExists
}
