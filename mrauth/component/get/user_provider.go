package get

import (
	"context"

	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-webcore/mraccess"
	"github.com/mondegor/go-webcore/mraccess/user"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
)

type (
	// UserProvider - компонент для извлечения настроек, которые хранятся в хранилище данных.
	UserProvider struct {
		storage       mrauth.AuthTokenFetcher
		errorWrapper  mrerr.UseCaseErrorWrapper
		allowedRealms map[string]struct{}
	}
)

// New - создаёт объект UserProvider.
func New(
	storage mrauth.AuthTokenFetcher,
	errorWrapper mrerr.UseCaseErrorWrapper,
	allowedRealms []string,
) *UserProvider {
	allowedRealmsMap := make(map[string]struct{}, len(allowedRealms))

	for _, allowedRealm := range allowedRealms {
		allowedRealmsMap[allowedRealm] = struct{}{}
	}

	return &UserProvider{
		storage:       storage,
		errorWrapper:  mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameAuthToken),
		allowedRealms: allowedRealmsMap,
	}
}

// MemberByToken - возвращает строковое значение настройки с указанным идентификатором.
func (co *UserProvider) MemberByToken(ctx context.Context, value string) (mraccess.Member, error) {
	if value == "" {
		return nil, mr.ErrUseCaseIncorrectInternalInputData.New("token is empty")
	}

	authToken, err := co.storage.FetchOne(ctx, value)
	if err != nil {
		if repository.ErrTokenExpired.Is(err) || co.errorWrapper.IsNotFoundOrNotAffectedError(err) {
			// возвращаемая ошибка специально обобщается
			return nil, mrauth.ErrTokenNotFoundOrExpired.New()
		}

		return nil, co.errorWrapper.WrapErrorFailed(err) // "token", trim value[:8]...
	}

	if _, ok := co.allowedRealms[authToken.Realm]; !ok {
		return nil, mr.ErrUseCaseAccessForbidden.New()
	}

	return user.New(authToken.UserID, authToken.Realm+"/"+authToken.UserKind, authToken.LangCode), nil
}
