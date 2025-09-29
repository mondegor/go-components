package mrauth

import (
	"strings"

	"github.com/mondegor/go-components/mrauth/component/get"
)

// NewUserProvider - создаёт получателя произвольных настроек из БД.
func NewUserProvider(providers ...get.ProviderWithTokenType) *get.UserProviderGroup {
	return get.NewGroup(
		func(token string) string {
			if token == "" {
				return ""
			}

			if strings.Count(token, ".") == 2 {
				return "jwt"
			}

			return "session"
		},
		providers,
	)
}
