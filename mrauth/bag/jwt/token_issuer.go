package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// TokenIssuer - компонент для извлечения настроек, которые хранятся в хранилище данных.
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
func (uc *TokenIssuer) Create(realm, userKind, langCode string, userID uuid.UUID) (token dto.AuthToken, err error) {
	scopes := dto.AuthTokenScopes{
		Realm:    realm,
		UserKind: userKind,
		LangCode: langCode,
		UserID:   userID,
	}

	accessToken, err := uc.createAccessToken(&scopes)
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
		Scopes:           scopes,
	}, nil
}

// Create - возвращает строковое значение настройки с указанным идентификатором.
func (uc *TokenIssuer) createAccessToken(scopes *dto.AuthTokenScopes) (string, error) {
	token := jwt.NewWithClaims(
		uc.signingMethod,
		jwt.MapClaims{
			sectionAudiences: scopes.Realm,
			sectionUserID:    scopes.UserID.String(),
			sectionLangCode:  scopes.LangCode,
			sectionScope:     scopes.UserKind,
			sectionExpiry:    jwt.NewNumericDate(time.Now().Add(uc.accessExpiry)),
		},
	)

	return token.SignedString(uc.secret)
}
