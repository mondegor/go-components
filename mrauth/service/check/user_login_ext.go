package check

import (
	"context"

	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
)

type (
	// UserLoginExt - сервис для проверки доступности логина пользователя (емаила или телефона).
	UserLoginExt struct {
		*UserLogin
		loginParser loginParser
	}

	loginParser interface {
		Parse(login string) (contactaddress.ContactAddress, error)
	}
)

// NewUserLoginExt - создаёт объект UserLoginExt.
func NewUserLoginExt(
	storageCheckUser checkUserStorage,
	storageUserRealm userRealmStorage,
	loginParser loginParser,
) *UserLoginExt {
	return &UserLoginExt{
		UserLogin: NewUserLogin(
			storageCheckUser,
			storageUserRealm,
		),
		loginParser: loginParser,
	}
}

// CheckAvailability - проверяет, что указанный логин не существует ни в одном realm.
func (s *UserLoginExt) CheckAvailability(ctx context.Context, userLogin string) error {
	return s.CheckAvailabilityRealm(ctx, "", userLogin)
}

// CheckAvailabilityRealm - проверяет, что указанный логин не существует в указанном realm.
func (s *UserLoginExt) CheckAvailabilityRealm(ctx context.Context, realm, userLogin string) error {
	parsedLogin, err := s.loginParser.Parse(userLogin)
	if err != nil {
		return contactaddress.ErrLoginIsInvalid.New()
	}

	if parsedLogin.Type == addresstype.Email {
		return s.CheckAvailabilityRealmEmail(ctx, realm, parsedLogin.Value)
	}

	if parsedLogin.Type == addresstype.Phone {
		return s.CheckAvailabilityRealmPhone(ctx, realm, parsedLogin.Value)
	}

	return contactaddress.ErrLoginIsInvalid.New()
}
