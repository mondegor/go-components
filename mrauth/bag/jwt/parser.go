package jwt

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

const (
	sectionAudiences = "aud"
	sectionUserID    = "sub"
	sectionLangCode  = "lan"
	sectionScope     = "scope"
	sectionExpiry    = "exp"
)

type (
	// Parser - comment struct.
	Parser struct {
		secret []byte
	}
)

var (
	// ErrTokenInvalid - token is invalid.
	ErrTokenInvalid = errors.NewUserProto("TokenInvalid", "jwt token is invalid")

	// ErrTokenSectionInvalid - token section is invalid.
	ErrTokenSectionInvalid = errors.NewUserProto("TokenSectionInvalid", "jwt token section '{Key}' is invalid")

	// ErrTokenExpired - token is expired.
	ErrTokenExpired = errors.NewUserProto("TokenExpired", "jwt token is expired")
)

// NewParser - создаёт объект Parser.
func NewParser(secret string) *Parser {
	return &Parser{
		secret: []byte(secret),
	}
}

// Parse - возвращает строковое значение настройки с указанным идентификатором.
func (p *Parser) Parse(value string) (dto.AuthTokenScopes, error) {
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(value, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return p.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return dto.AuthTokenScopes{}, ErrTokenExpired
		}

		return dto.AuthTokenScopes{}, ErrTokenInvalid.Wrap(err)
	}

	if !token.Valid {
		return dto.AuthTokenScopes{}, ErrTokenInvalid
	}

	realm, err := p.parseString(sectionAudiences, claims)
	if err != nil {
		return dto.AuthTokenScopes{}, ErrTokenSectionInvalid.Wrap(err, sectionAudiences)
	}

	userID, err := p.parseUserID(claims)
	if err != nil {
		return dto.AuthTokenScopes{}, ErrTokenSectionInvalid.Wrap(err, sectionUserID)
	}

	langCode, err := p.parseString(sectionLangCode, claims)
	if err != nil {
		return dto.AuthTokenScopes{}, ErrTokenSectionInvalid.Wrap(err, sectionLangCode)
	}

	scope, err := p.parseString(sectionScope, claims)
	if err != nil {
		return dto.AuthTokenScopes{}, ErrTokenSectionInvalid.Wrap(err, sectionScope)
	}

	return dto.AuthTokenScopes{
		Realm:    realm,
		UserKind: scope,
		LangCode: langCode,
		UserID:   userID,
	}, nil
}

func (p *Parser) parseUserID(claims map[string]any) (uuid.UUID, error) {
	id, err := p.parseString(sectionUserID, claims)
	if err != nil {
		return uuid.Nil, err
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, errors.New("userID is invalid; expected: uuid type")
	}

	return userID, nil
}

func (p *Parser) parseString(key string, claims map[string]any) (string, error) {
	raw, ok := claims[key]
	if !ok {
		return "", nil
	}

	str, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("claims[%s] is invalid; expected: string type", key)
	}

	return str, nil
}
