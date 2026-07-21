package jwt

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

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

var (
	// ErrTokenInvalid - token is invalid.
	ErrTokenInvalid = errors.NewUserError("TokenInvalid", "jwt token is invalid")

	// ErrTokenSectionInvalid - token section is invalid.
	ErrTokenSectionInvalid = errors.NewUserProto("TokenSectionInvalid", "jwt token section '{Key}' is invalid")

	// ErrTokenExpired - token is expired.
	ErrTokenExpired = errors.NewUserError("TokenExpired", "jwt token is expired")
)

// NewParser - создаёт объект Parser с набором ключей для проверки подписи.
func NewParser(keys crypt.KeySet) *Parser {
	return &Parser{
		keys: keys,
	}
}

// Parse - проверяет подпись access-токена и возвращает извлечённую область действия пользователя.
func (p *Parser) Parse(value string) (dto.UserScopes, error) {
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(value, claims, func(token *jwt.Token) (any, error) {
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

	timeZone, err := p.parseString(sectionTimeZone, claims)
	if err != nil {
		return dto.UserScopes{}, ErrTokenSectionInvalid.Wrap(err, sectionTimeZone)
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
		return "", fmt.Errorf("claims[%s] is missing", key)
	}

	str, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("claims[%s] is invalid; expected: string type", key)
	}

	if str == "" {
		return "", fmt.Errorf("claims[%s] is empty", key)
	}

	return str, nil
}
