package get

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mraccess"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/repository"
)

//go:generate go tool mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth AuthTokenFetcher
//go:generate go tool mockgen -destination=mock/mraccess.go -package=mock github.com/mondegor/go-sysmess/mraccess RightsGetter

type (
	// UserProvider - comment struct.
	UserProvider struct {
		storage          mrauth.AuthTokenFetcher
		errorWrapper     errors.Wrapper
		userGroupRights  mraccess.RightsGetter
		allowedRealmsMap map[string]bool
	}
)

// New - создаёт объект UserProvider.
func New(
	storage mrauth.AuthTokenFetcher,
	userGroupRights mraccess.RightsGetter,
	allowedRealms []string,
) *UserProvider {
	allowedRealmsMap := make(map[string]bool, len(allowedRealms))

	for _, allowedRealm := range allowedRealms {
		allowedRealmsMap[allowedRealm] = true
	}

	return &UserProvider{
		storage:          storage,
		errorWrapper:     errors.NewServiceOperationFailedWrapper(),
		userGroupRights:  userGroupRights,
		allowedRealmsMap: allowedRealmsMap,
	}
}

// UserByToken - возвращает пользователя с его правами доступа по access токену.
func (co *UserProvider) UserByToken(ctx context.Context, value string) (mraccess.User, error) {
	if value == "" {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("token is empty")
	}

	userScopes, err := co.storage.FetchOneByAccessToken(ctx, value)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) || errors.Is(err, repository.ErrTokenExpired) {
			return nil, mrauth.ErrTokenNotFoundOrExpired // новая ошибка специально обобщает
		}

		return nil, co.errorWrapper.Wrap(err) // "token", trim value[:8]...
	}

	if !co.allowedRealmsMap[userScopes.Realm] {
		return nil, errors.ErrAccessForbidden
	}

	return mraccess.NewUser(
		userScopes.UserID,
		userScopes.Realm+"/"+userScopes.Kind,
		userScopes.LangCode,
		co.userGroupRights,
	), nil
}
