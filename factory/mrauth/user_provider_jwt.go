package mrauth

import (
	"github.com/mondegor/go-sysmess/mrerr"

	"github.com/mondegor/go-components/mrauth/component/get"
	"github.com/mondegor/go-components/mrauth/repository"
)

// NewUserProviderJWT - создаёт получателя произвольных настроек из БД.
func NewUserProviderJWT(
	useCaseErrorWrapper mrerr.UseCaseErrorWrapper,
	secret string,
	allowedRealms []string,
) *get.UserProvider {
	return get.New(
		repository.NewAuthTokenJWT(secret),
		useCaseErrorWrapper,
		allowedRealms,
	)
}
