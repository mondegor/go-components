package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// TokenIssuer - comment struct.
	TokenIssuer struct {
		tokenGenerator mrauth.TokenGenerator
		accessExpiry   time.Duration
		refreshExpiry  time.Duration
		signingMethod  jwt.SigningMethod
		secret         []byte
	}
)

// NewTokenIssuer - создаёт объект Session.
func NewTokenIssuer(
	tokenGenerator mrauth.TokenGenerator,
	accessExpiry time.Duration,
	refreshExpiry time.Duration,
	signingMethod string,
	secret []byte,
) *TokenIssuer {
	var method jwt.SigningMethod

	switch signingMethod {
	case "HS512":
		method = jwt.SigningMethodHS512
	// TODO: "ES256", "ES512"
	// case "ES256":
	// 	method = jwt.SigningMethodES256
	// case "ES512":
	// 	method = jwt.SigningMethodES512
	default:
		method = jwt.SigningMethodHS256
	}

	return &TokenIssuer{
		tokenGenerator: tokenGenerator,
		accessExpiry:   accessExpiry,
		refreshExpiry:  refreshExpiry,
		signingMethod:  method,
		secret:         secret,
	}
}

// Create - comments method.
func (uc *TokenIssuer) Create(userScopes dto.UserScopes) (token dto.AuthToken, err error) {
	accessToken, err := uc.createAccessToken(&userScopes)
	if err != nil {
		return dto.AuthToken{}, err
	}

	refreshToken, err := uc.tokenGenerator.GenToken()
	if err != nil {
		return dto.AuthToken{}, err
	}

	return dto.AuthToken{
		AccessToken:      accessToken,
		ExpiresIn:        uc.accessExpiry,
		HasSignature:     true,
		RefreshToken:     refreshToken,
		RefreshExpiresIn: uc.refreshExpiry,
		UserID:           userScopes.UserID,
		Scopes: entity.AuthTokenScopes{
			Realm:    userScopes.Realm,
			UserKind: userScopes.Kind,
			LangCode: userScopes.LangCode,
		},
	}, nil
}

// Create - возвращает строковое значение настройки с указанным идентификатором.
func (uc *TokenIssuer) createAccessToken(userScopes *dto.UserScopes) (string, error) {
	token := jwt.NewWithClaims(
		uc.signingMethod,
		jwt.MapClaims{
			sectionAudiences: userScopes.Realm,
			sectionUserID:    userScopes.UserID.String(),
			sectionLangCode:  userScopes.LangCode,
			sectionScope:     userScopes.Kind,
			sectionExpiry:    jwt.NewNumericDate(time.Now().Add(uc.accessExpiry)),
		},
	)

	return token.SignedString(uc.secret)
}
