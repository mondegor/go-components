package mrauth

import (
	"strings"

	"github.com/mondegor/go-core/mraccess"
)

// NewUserProvider - создаёт получателя произвольных настроек из БД.
func NewUserProvider(providers ...mraccess.TypedUserProvider) mraccess.UserProvider {
	return mraccess.NewUserProviderGroup(
		providers,
		func(token string) string {
			if token == "" {
				return ""
			}

			// JWT состоит из трёх частей через точку: header.payload.signature
			if strings.Count(token, ".") == 2 {
				return "jwt"
			}

			return "session"
		},
	)
}
