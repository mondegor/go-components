package check

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
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
		errorWrapper:     errors.NewServiceWrapper(),
	}
}

// CheckAvailability - проверяет, что указанный логин не существует ни в одном realm.
func (s *UserLogin) CheckAvailability(ctx context.Context, userLogin contactaddress.ContactAddress) error {
	return s.CheckAvailabilityRealm(ctx, "", userLogin)
}

// CheckAvailabilityRealm - проверяет, что указанный логин не существует в указанном realm.
func (s *UserLogin) CheckAvailabilityRealm(ctx context.Context, realm string, userLogin contactaddress.ContactAddress) error {
	if userLogin.Type == addresstype.Email {
		return s.CheckAvailabilityRealmEmail(ctx, realm, userLogin.Value)
	}

	if userLogin.Type == addresstype.Phone {
		return s.CheckAvailabilityRealmPhone(ctx, realm, userLogin.Value)
	}

	return contactaddress.ErrLoginIsInvalid
}

// CheckAvailabilityEmail - проверяет, что указанный email не существует ни в одном realm.
// Если email существует, то вернётся mrauth.ErrEmailAlreadyExists.
func (s *UserLogin) CheckAvailabilityEmail(ctx context.Context, userEmail string) error {
	return s.CheckAvailabilityRealmEmail(ctx, "", userEmail)
}

// CheckAvailabilityRealmEmail - проверяет, что указанный email не существует в указанном realm.
// Если email существует, то вернётся mrauth.ErrEmailAlreadyExists.
func (s *UserLogin) CheckAvailabilityRealmEmail(ctx context.Context, realm, userEmail string) error {
	userID, err := s.storageCheckUser.UserIDByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRowFound) {
			return nil
		}

		return s.errorWrapper.Wrap(err)
	}

	if realm == "" {
		return mrauth.ErrEmailAlreadyExists
	}

	return s.checkUserRealm(ctx, userID, realm, mrauth.ErrEmailAlreadyExists)
}

// CheckAvailabilityPhone - проверяет, что указанный телефон не существует ни в одном realm.
// Если телефон существует, то вернётся mrauth.ErrPhoneAlreadyExists.
func (s *UserLogin) CheckAvailabilityPhone(ctx context.Context, userPhone string) error {
	return s.CheckAvailabilityRealmPhone(ctx, "", userPhone)
}

// CheckAvailabilityRealmPhone - проверяет, что указанный телефон не существует в указанном realm.
// Если телефон существует, то вернётся mrauth.ErrPhoneAlreadyExists.
func (s *UserLogin) CheckAvailabilityRealmPhone(ctx context.Context, realm, userPhone string) error {
	parsedPhone, err := strconv.ParseUint(userPhone, 10, 64)
	if err != nil {
		return contactaddress.ErrPhoneIsInvalid
	}

	userID, err := s.storageCheckUser.UserIDByPhone(ctx, parsedPhone)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRowFound) {
			return nil
		}

		return s.errorWrapper.Wrap(err)
	}

	if realm == "" {
		return mrauth.ErrPhoneAlreadyExists
	}

	return s.checkUserRealm(ctx, userID, realm, mrauth.ErrPhoneAlreadyExists)
}

func (s *UserLogin) checkUserRealm(ctx context.Context, userID uuid.UUID, realm string, errIfExists error) error {
	if _, err := s.storageUserRealm.FetchOne(ctx, userID, realm); err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRowFound) {
			return nil
		}

		return s.errorWrapper.Wrap(err)
	}

	return errIfExists
}
