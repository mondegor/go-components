package crypt

import (
	"crypto/rsa"

	"github.com/golang-jwt/jwt/v5"
)

type (
	// rsaKey - асимметричный ключ подписи на базе RSA (RS256).
	rsaKey struct {
		kid     string
		private *rsa.PrivateKey
	}

	// rsaVerifyKey - RSA-ключ только для проверки подписи (без приватной части),
	// используется для старых ключей в период ротации.
	rsaVerifyKey struct {
		kid    string
		public *rsa.PublicKey
	}
)

// NewRSAKey - создаёт RSA-ключ подписи (RS256) с указанным идентификатором.
func NewRSAKey(kid string, private *rsa.PrivateKey) SigningKey {
	return rsaKey{
		kid:     kid,
		private: private,
	}
}

// KID - возвращает идентификатор ключа.
func (k rsaKey) KID() string { return k.kid }

// Method - возвращает алгоритм подписи (RS256).
func (k rsaKey) Method() jwt.SigningMethod { return jwt.SigningMethodRS256 }

// Private - возвращает приватный ключ для подписи токена.
func (k rsaKey) Private() any { return k.private }

// Public - возвращает публичный ключ для проверки подписи.
func (k rsaKey) Public() any { return &k.private.PublicKey }

// NewRSAVerifyKey - создаёт RSA-ключ только для проверки подписи (RS256) с указанным идентификатором.
func NewRSAVerifyKey(kid string, public *rsa.PublicKey) Key {
	return rsaVerifyKey{
		kid:    kid,
		public: public,
	}
}

// KID - возвращает идентификатор ключа.
func (k rsaVerifyKey) KID() string { return k.kid }

// Method - возвращает алгоритм подписи (RS256).
func (k rsaVerifyKey) Method() jwt.SigningMethod { return jwt.SigningMethodRS256 }

// Public - возвращает публичный ключ для проверки подписи.
func (k rsaVerifyKey) Public() any { return k.public }
