package get

import (
	"context"

	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-webcore/mraccess"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/repository"
)

type (
	// UserProvider - comment struct.
	UserProvider struct {
		storage       mrauth.AuthTokenFetcher
		errorWrapper  mrerr.UseCaseErrorWrapper
		userGroups    mraccess.RightsGetter
		allowedRealms map[string]struct{}
	}
)

// New - создаёт объект UserProvider.
func New(
	storage mrauth.AuthTokenFetcher,
	errorWrapper mrerr.UseCaseErrorWrapper,
	userGroups mraccess.RightsGetter,
	allowedRealms []string,
) *UserProvider {
	allowedRealmsMap := make(map[string]struct{}, len(allowedRealms))

	for _, allowedRealm := range allowedRealms {
		allowedRealmsMap[allowedRealm] = struct{}{}
	}

	return &UserProvider{
		storage:       storage,
		errorWrapper:  mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.UserProvider"),
		userGroups:    userGroups,
		allowedRealms: allowedRealmsMap,
	}
}

// UserByToken - возвращает строковое значение настройки с указанным идентификатором.
func (co *UserProvider) UserByToken(ctx context.Context, value string) (mraccess.User, error) {
	if value == "" {
		return nil, mr.ErrUseCaseIncorrectInternalInputData.New("token is empty")
	}

	authToken, err := co.storage.FetchOne(ctx, value)
	if err != nil {
		if repository.ErrTokenExpired.Is(err) || co.errorWrapper.IsNotFoundError(err) {
			// возвращаемая ошибка специально обобщается
			return nil, mrauth.ErrTokenNotFoundOrExpired.Wrap(mr.ErrUseCaseEntityNotFound)
		}

		return nil, co.errorWrapper.WrapErrorFailed(err) // "token", trim value[:8]...
	}

	if _, ok := co.allowedRealms[authToken.Realm]; !ok {
		return nil, mr.ErrUseCaseAccessForbidden.New()
	}

	return mraccess.NewUser(
		authToken.UserID,
		authToken.Realm+"/"+authToken.UserKind,
		authToken.LangCode,
		co.userGroups,
	), nil
}
