package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

//go:generate go tool mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth TokenGenerator

type (
	// TokenIssuer - выпускает пару токенов с подписанным (JWT) access токеном.
	TokenIssuer struct {
		tokenGenerator mrauth.TokenGenerator
		accessExpiry   time.Duration
		refreshExpiry  time.Duration
		signingMethod  jwt.SigningMethod
		secret         []byte
	}
)

// NewTokenIssuer - создаёт объект TokenIssuer.
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

// CreateTokenPair - выпускает пару токенов (подписанный JWT access + refresh) для области действия пользователя.
// TODO: вместо dto.UserScopes можно передавать явно все параметры.
func (uc *TokenIssuer) CreateTokenPair(userScopes dto.UserScopes) (token dto.AuthTokenPair, err error) {
	accessToken, err := uc.createAccessToken(&userScopes)
	if err != nil {
		return dto.AuthTokenPair{}, err
	}

	refreshToken, err := uc.createRefreshToken()
	if err != nil {
		return dto.AuthTokenPair{}, err
	}

	return dto.AuthTokenPair{
		Access:  accessToken,
		Refresh: refreshToken,
		UserID:  userScopes.UserID,
		Scopes: entity.AuthTokenScopes{
			Realm:    userScopes.Realm,
			UserKind: userScopes.Kind,
			LangCode: userScopes.LangCode,
		},
	}, nil
}

func (uc *TokenIssuer) createAccessToken(userScopes *dto.UserScopes) (dto.AccessToken, error) {
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

	accessToken, err := token.SignedString(uc.secret)
	if err != nil {
		return dto.AccessToken{}, err
	}

	return dto.AccessToken{
		Token:        accessToken,
		ExpiresIn:    uc.accessExpiry,
		HasSignature: true,
	}, nil
}

func (uc *TokenIssuer) createRefreshToken() (token dto.RefreshToken, err error) {
	refreshToken, err := uc.tokenGenerator.GenToken()
	if err != nil {
		return dto.RefreshToken{}, err
	}

	return dto.RefreshToken{
		Token:     refreshToken,
		ExpiresIn: uc.refreshExpiry,
	}, nil
}
