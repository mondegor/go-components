package jwt

import (
	"fmt"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

const (
	sectionAudiences = "aud"
	sectionUserID    = "sub"
	sectionSessionID = "sid"
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
	ErrTokenInvalid = errors.NewUserError("TokenInvalid", "jwt token is invalid")

	// ErrTokenSectionInvalid - token section is invalid.
	ErrTokenSectionInvalid = errors.NewUserProto("TokenSectionInvalid", "jwt token section '{Key}' is invalid")

	// ErrTokenExpired - token is expired.
	ErrTokenExpired = errors.NewUserError("TokenExpired", "jwt token is expired")
)

// NewParser - создаёт объект Parser.
func NewParser(secret string) *Parser {
	return &Parser{
		secret: []byte(secret),
	}
}

// Parse - возвращает строковое значение настройки с указанным идентификатором.
func (p *Parser) Parse(value string) (dto.UserScopes, error) {
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(value, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return p.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return dto.UserScopes{}, ErrTokenExpired
		}

		return dto.UserScopes{}, ErrTokenInvalid.Wrap(err)
	}

	if !token.Valid {
		return dto.UserScopes{}, ErrTokenInvalid
	}

	realm, err := p.parseString(sectionAudiences, claims)
	if err != nil {
		return dto.UserScopes{}, ErrTokenSectionInvalid.Wrap(err, sectionAudiences)
	}

	userID, err := p.parseUserID(claims)
	if err != nil {
		return dto.UserScopes{}, ErrTokenSectionInvalid.Wrap(err, sectionUserID)
	}

	sessionID, err := p.parseSessionID(claims)
	if err != nil {
		return dto.UserScopes{}, ErrTokenSectionInvalid.Wrap(err, sectionSessionID)
	}

	langCode, err := p.parseString(sectionLangCode, claims)
	if err != nil {
		return dto.UserScopes{}, ErrTokenSectionInvalid.Wrap(err, sectionLangCode)
	}

	scope, err := p.parseString(sectionScope, claims)
	if err != nil {
		return dto.UserScopes{}, ErrTokenSectionInvalid.Wrap(err, sectionScope)
	}

	return dto.UserScopes{
		UserID:    userID,
		SessionID: sessionID,
		Realm:     realm,
		Kind:      scope,
		LangCode:  langCode,
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

func (p *Parser) parseSessionID(claims map[string]any) (uint32, error) {
	raw, err := p.parseString(sectionSessionID, claims)
	if err != nil {
		return 0, err
	}

	sessionID, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, errors.New("sessionID is invalid; expected: uint32 type")
	}

	return uint32(sessionID), nil
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
