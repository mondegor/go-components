package crypt

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

type (
	// hmacKey - симметричный (HMAC) ключ подписи.
	hmacKey struct {
		kid    string
		method jwt.SigningMethod
		secret []byte
	}
)

// NewHMACKey - создаёт симметричный (HMAC) ключ подписи с указанным методом
// (пустой метод трактуется как HS256 по умолчанию); неподдерживаемый метод - ошибка.
func NewHMACKey(kid, method string, secret []byte) (SigningKey, error) {
	var signingMethod jwt.SigningMethod

	switch method {
	case "", "HS256":
		signingMethod = jwt.SigningMethodHS256
	case "HS512":
		signingMethod = jwt.SigningMethodHS512
	default:
		return nil, fmt.Errorf("unsupported HMAC signing method: %q", method)
	}

	return hmacKey{
		kid:    kid,
		method: signingMethod,
		secret: secret,
	}, nil
}

// KID - возвращает идентификатор ключа.
func (k hmacKey) KID() string { return k.kid }

// Method - возвращает алгоритм подписи (HS256/HS512).
func (k hmacKey) Method() jwt.SigningMethod { return k.method }

// Private - возвращает секрет для подписи токена.
func (k hmacKey) Private() any { return k.secret }

// Public - возвращает секрет для проверки подписи (у HMAC совпадает с приватным).
func (k hmacKey) Public() any { return k.secret }
