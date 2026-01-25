package get

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-webcore/mraccess"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/repository"
)

type (
	// UserProvider - comment struct.
	UserProvider struct {
		storage          mrauth.AuthTokenFetcher
		errorWrapper     errors.Wrapper
		userGroups       mraccess.RightsGetter
		allowedRealmsMap map[string]bool
	}
)

// New - создаёт объект UserProvider.
func New(
	storage mrauth.AuthTokenFetcher,
	userGroups mraccess.RightsGetter,
	allowedRealms []string,
) *UserProvider {
	allowedRealmsMap := make(map[string]bool, len(allowedRealms))

	for _, allowedRealm := range allowedRealms {
		allowedRealmsMap[allowedRealm] = true
	}

	return &UserProvider{
		storage:          storage,
		errorWrapper:     errors.NewServiceWrapper(),
		userGroups:       userGroups,
		allowedRealmsMap: allowedRealmsMap,
	}
}

// UserByToken - возвращает строковое значение настройки с указанным идентификатором.
func (co *UserProvider) UserByToken(ctx context.Context, value string) (mraccess.User, error) {
	if value == "" {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("token is empty")
	}

	authToken, err := co.storage.FetchOne(ctx, value)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRowFound) || errors.Is(err, repository.ErrTokenExpired) {
			return nil, mrauth.ErrTokenNotFoundOrExpired
		}

		return nil, co.errorWrapper.Wrap(err) // "token", trim value[:8]...
	}

	if !co.allowedRealmsMap[authToken.Realm] {
		return nil, errors.ErrUseCaseAccessForbidden
	}

	return mraccess.NewUser(
		authToken.UserID,
		authToken.Realm+"/"+authToken.UserKind,
		authToken.LangCode,
		co.userGroups,
	), nil
}
