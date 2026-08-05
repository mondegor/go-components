package jwt

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
	"github.com/mondegor/go-components/mrauth/dto"
)

const (
	sectionAudiences = "aud"
	sectionUserID    = "sub"
	sectionSessionID = "sid"
	sectionLangCode  = "lan"
	sectionTimeZone  = "tz"
	sectionScope     = "scope"
	sectionIssuer    = "iss"
	sectionIssuedAt  = "iat"
	sectionExpiry    = "exp"
	sectionJTI       = "jti"

	parseLeeway = 45 * time.Second // допустимое расхождение часов при проверке exp/nbf/iat
)

type (
	// Parser - разбирает и проверяет подпись access-токена, извлекая область действия пользователя.
	Parser struct {
		keys crypt.KeySet
	}
)

// ErrTokenExpired - token is expired.
var ErrTokenExpired = errors.New("jwt token is expired")

// NewParser - создаёт объект Parser с набором ключей для проверки подписи.
func NewParser(keys crypt.KeySet) *Parser {
	return &Parser{
		keys: keys,
	}
}

// Parse - проверяет подпись access-токена и возвращает извлечённую область действия пользователя.
func (p *Parser) Parse(value string) (dto.UserScopes, error) {
	claims := jwt.MapClaims{}

	_, err := jwt.ParseWithClaims(value, claims, func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)

		key, ok := p.keys.KeyByKID(kid)
		if !ok {
			return nil, fmt.Errorf("unknown key id: %q", kid)
		}

		// точный пин алгоритма по 'alg': отклоняет 'alg: none', HS↔RS confusion
		// и подмену внутри семейства (например HS256 вместо HS512)
		if token.Method.Alg() != key.Method().Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return key.Public(), nil
	}, jwt.WithLeeway(parseLeeway))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return dto.UserScopes{}, ErrTokenExpired
		}

		return dto.UserScopes{}, fmt.Errorf("jwt token is not parsed; %w", err)
	}

	realm, err := p.parseString(sectionAudiences, claims)
	if err != nil {
		return dto.UserScopes{}, err
	}

	userID, err := p.parseUserID(claims)
	if err != nil {
		return dto.UserScopes{}, err
	}

	sessionID, err := p.parseSessionID(claims)
	if err != nil {
		return dto.UserScopes{}, err
	}

	langCode, err := p.parseString(sectionLangCode, claims)
	if err != nil {
		return dto.UserScopes{}, err
	}

	timeZone, err := p.parseString(sectionTimeZone, claims)
	if err != nil {
		return dto.UserScopes{}, err
	}

	scope, err := p.parseString(sectionScope, claims)
	if err != nil {
		return dto.UserScopes{}, err
	}

	return dto.UserScopes{
		UserID:    userID,
		SessionID: sessionID,
		Realm:     realm,
		Kind:      scope,
		LangCode:  langCode,
		TimeZone:  timeZone,
	}, nil
}

func (p *Parser) parseUserID(claims map[string]any) (uuid.UUID, error) {
	id, err := p.parseString(sectionUserID, claims)
	if err != nil {
		return uuid.Nil, err
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, errors.New("jwt token userID is invalid; expected: uuid type")
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
		return 0, errors.New("jwt token sessionID is invalid; expected: uint32 type")
	}

	return uint32(sessionID), nil
}

func (p *Parser) parseString(key string, claims map[string]any) (string, error) {
	raw, ok := claims[key]
	if !ok {
		return "", fmt.Errorf("jwt token claims[%s] is missing", key)
	}

	str, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("jwt token claims[%s] is invalid; expected: string type", key)
	}

	if str == "" {
		return "", fmt.Errorf("jwt token claims[%s] is empty", key)
	}

	return str, nil
}
