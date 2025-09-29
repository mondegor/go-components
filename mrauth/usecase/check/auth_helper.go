package check

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrerrors"
	"github.com/mondegor/go-sysmess/mrlib/crypt"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
)

type (
	// AuthHelper - репозиторий для хранения сообщений подготовленных для отправки различным получателям.
	AuthHelper struct {
		storageCheckUser mrauth.CheckUserStorage
		storageUserRealm mrauth.UserRealmStorage
		loginParser      loginParser
		errorWrapper     core.UseCaseErrorWrapper
	}

	loginParser interface {
		Parse(login string) (contactaddress.ContactAddress, error)
	}
)

// NewAuthHelper - создаёт объект AuthHelper.
func NewAuthHelper(storageCheckUser mrauth.CheckUserStorage, storageUserRealm mrauth.UserRealmStorage, loginParser loginParser) *AuthHelper {
	return &AuthHelper{
		storageCheckUser: storageCheckUser,
		storageUserRealm: storageUserRealm,
		loginParser:      loginParser,
		errorWrapper:     core.NewUseCaseErrorWrapper(entity.ModelNameUser),
	}
}

// CheckAvailability - comments method.
func (uc *AuthHelper) CheckAvailability(ctx context.Context, realm, userLogin string) error {
	parsedLogin, err := uc.loginParser.Parse(userLogin)
	if err != nil {
		return uc.errorWrapper.WrapErrorFailed(err)
	}

	if parsedLogin.Type == enum.AddressTypeEmail {
		return uc.checkAvailabilityByEmail(ctx, realm, parsedLogin.Value)
	}

	if parsedLogin.Type == enum.AddressTypePhone {
		return uc.checkAvailabilityByPhone(ctx, realm, parsedLogin.Value)
	}

	return contactaddress.ErrLoginIsInvalid.New()
}

// CheckAvailabilityEmail - comments method.
func (uc *AuthHelper) CheckAvailabilityEmail(ctx context.Context, userEmail string) error {
	return uc.checkAvailabilityByEmail(ctx, "", userEmail)
}

// CheckAvailabilityPhone - comments method.
func (uc *AuthHelper) CheckAvailabilityPhone(ctx context.Context, userPhone string) error {
	return uc.checkAvailabilityByPhone(ctx, "", userPhone)
}

// CheckPasswordStrength - comments method.
func (uc *AuthHelper) CheckPasswordStrength(_ context.Context, password string) (crypt.PassStrength, error) {
	return crypt.PasswordStrength(password), nil
}

func (uc *AuthHelper) checkAvailabilityByEmail(ctx context.Context, realm, userEmail string) error {
	userID, err := uc.storageCheckUser.UserIDByEmail(ctx, userEmail)
	if err != nil {
		if mr.ErrStorageNoRowFound.Is(err) {
			return nil
		}

		return uc.errorWrapper.WrapErrorFailed(err)
	}

	if realm == "" {
		return mrauth.ErrEmailAlreadyExists.New()
	}

	return uc.checkUserRealm(ctx, userID, realm, mrauth.ErrEmailAlreadyExists)
}

func (uc *AuthHelper) checkAvailabilityByPhone(ctx context.Context, realm, userPhone string) error {
	parsedPhone, err := strconv.ParseUint(userPhone, 10, 64)
	if err != nil {
		return contactaddress.ErrPhoneIsInvalid.New()
	}

	userID, err := uc.storageCheckUser.UserIDByPhone(ctx, parsedPhone)
	if err != nil {
		if mr.ErrStorageNoRowFound.Is(err) {
			return nil
		}

		return uc.errorWrapper.WrapErrorFailed(err)
	}

	if realm == "" {
		return mrauth.ErrPhoneAlreadyExists.New()
	}

	return uc.checkUserRealm(ctx, userID, realm, mrauth.ErrPhoneAlreadyExists)
}

func (uc *AuthHelper) checkUserRealm(ctx context.Context, userID uuid.UUID, realm string, errIfExists *mrerrors.ProtoError) error {
	if _, err := uc.storageUserRealm.FetchOne(ctx, userID, realm); err != nil {
		if mr.ErrStorageNoRowFound.Is(err) {
			return nil
		}

		return uc.errorWrapper.WrapErrorFailed(err)
	}

	return errIfExists.New()
}
