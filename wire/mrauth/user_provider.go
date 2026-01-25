package mrauth

import (
	"strings"

	"github.com/mondegor/go-webcore/mraccess"
)

// NewUserProvider - создаёт получателя произвольных настроек из БД.
func NewUserProvider(providers ...mraccess.TypedUserProvider) mraccess.UserProvider {
	return mraccess.NewUserProviderGroup(
		providers,
		func(token string) string {
			if token == "" {
				return ""
			}

			if strings.Count(token, ".") == 2 {
				return "jwt"
			}

			return "session"
		},
	)
}
