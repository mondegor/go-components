package get

import (
	"context"

	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-webcore/mraccess"

	"github.com/mondegor/go-components/mrauth"
)

type (
	// UserProviderGroup - компонент для извлечения настроек, которые хранятся в хранилище данных.
	UserProviderGroup struct {
		type2provider     map[string]mraccess.MemberProvider
		identifyTokenType func(token string) string
	}

	// ProviderWithTokenType - comment struct.
	ProviderWithTokenType struct {
		TokenType string
		Provider  mraccess.MemberProvider
	}
)

// NewGroup - создаёт объект UserProviderGroup.
func NewGroup(identifyTokenType func(token string) string, providers []ProviderWithTokenType) *UserProviderGroup {
	type2provider := make(map[string]mraccess.MemberProvider, len(providers))

	for _, pr := range providers {
		type2provider[pr.TokenType] = pr.Provider
	}

	return &UserProviderGroup{
		identifyTokenType: identifyTokenType,
		type2provider:     type2provider,
	}
}

// MemberByToken - возвращает строковое значение настройки с указанным идентификатором.
func (co *UserProviderGroup) MemberByToken(ctx context.Context, value string) (mraccess.Member, error) {
	if value == "" {
		return nil, mr.ErrUseCaseIncorrectInternalInputData.New("token is empty")
	}

	if tp := co.identifyTokenType(value); tp != "" {
		if pr, ok := co.type2provider[tp]; ok {
			return pr.MemberByToken(ctx, value)
		}
	}

	return nil, mrauth.ErrTokenInvalid.New() // "value", value
}
