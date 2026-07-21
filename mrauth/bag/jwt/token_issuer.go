package jwt

import (
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

const (
	defaultAccessExpiry  = 5 * time.Minute // короткий TTL access-токена по умолчанию
	defaultRefreshExpiry = 24 * time.Hour
	defaultIssuer        = "mrauth"
)

type (
	// TokenIssuer - выпускает пару токенов с подписанным (JWT) access токеном.
	TokenIssuer struct {
		tokenGenerator mrauth.TokenGenerator
		accessExpiry   time.Duration
		refreshExpiry  time.Duration
		issuer         string
		signingKey     crypt.SigningKey
	}
)

// NewTokenIssuer - создаёт объект TokenIssuer.
func NewTokenIssuer(
	tokenGenerator mrauth.TokenGenerator,
	accessExpiry time.Duration,
	refreshExpiry time.Duration,
	issuer string,
	signingKey crypt.SigningKey,
) *TokenIssuer {
	if accessExpiry == 0 {
		accessExpiry = defaultAccessExpiry
	}

	if refreshExpiry == 0 {
		refreshExpiry = defaultRefreshExpiry
	}

	if issuer == "" {
		issuer = defaultIssuer
	}

	return &TokenIssuer{
		tokenGenerator: tokenGenerator,
		accessExpiry:   accessExpiry,
		refreshExpiry:  refreshExpiry,
		issuer:         issuer,
		signingKey:     signingKey,
	}
}

// CreateTokenPair - выпускает пару токенов (подписанный JWT access + refresh) для области действия пользователя.
// TODO: вместо dto.UserScopes можно передавать явно все параметры.
func (uc *TokenIssuer) CreateTokenPair(userScopes dto.UserScopes) (token dto.AuthTokenPair, err error) {
	if err = userScopes.Validate(); err != nil {
		return dto.AuthTokenPair{}, err
	}

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
			TimeZone: userScopes.TimeZone,
		},
	}, nil
}

func (uc *TokenIssuer) createAccessToken(userScopes *dto.UserScopes) (dto.AccessToken, error) {
	now := time.Now().UTC()

	token := jwt.NewWithClaims(
		uc.signingKey.Method(),
		jwt.MapClaims{
			sectionAudiences: userScopes.Realm,
			sectionUserID:    userScopes.UserID.String(),
			sectionSessionID: strconv.FormatUint(uint64(userScopes.SessionID), 10),
			sectionLangCode:  userScopes.LangCode,
			sectionTimeZone:  userScopes.TimeZone,
			sectionScope:     userScopes.Kind,
			sectionIssuer:    uc.issuer,
			sectionIssuedAt:  jwt.NewNumericDate(now),
			sectionExpiry:    jwt.NewNumericDate(now.Add(uc.accessExpiry)),
			sectionJTI:       uuid.NewString(),
		},
	)

	if kid := uc.signingKey.KID(); kid != "" {
		token.Header["kid"] = kid
	}

	accessToken, err := token.SignedString(uc.signingKey.Private())
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
