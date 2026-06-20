package crypt

import (
	"crypto/ecdsa"

	"github.com/golang-jwt/jwt/v5"
)

type (
	// ecdsaKey - асимметричный ключ подписи на базе ECDSA (ES256).
	ecdsaKey struct {
		kid     string
		private *ecdsa.PrivateKey
	}

	// ecdsaVerifyKey - ECDSA-ключ только для проверки подписи (без приватной части),
	// используется для старых ключей в период ротации.
	ecdsaVerifyKey struct {
		kid    string
		public *ecdsa.PublicKey
	}
)

// NewECDSAKey - создаёт ECDSA-ключ подписи (ES256) с указанным идентификатором.
func NewECDSAKey(kid string, private *ecdsa.PrivateKey) SigningKey {
	return ecdsaKey{
		kid:     kid,
		private: private,
	}
}

// KID - возвращает идентификатор ключа.
func (k ecdsaKey) KID() string { return k.kid }

// Method - возвращает алгоритм подписи (ES256).
func (k ecdsaKey) Method() jwt.SigningMethod { return jwt.SigningMethodES256 }

// Private - возвращает приватный ключ для подписи токена.
func (k ecdsaKey) Private() any { return k.private }

// Public - возвращает публичный ключ для проверки подписи.
func (k ecdsaKey) Public() any { return &k.private.PublicKey }

// NewECDSAVerifyKey - создаёт ECDSA-ключ только для проверки подписи (ES256) с указанным идентификатором.
func NewECDSAVerifyKey(kid string, public *ecdsa.PublicKey) Key {
	return ecdsaVerifyKey{
		kid:    kid,
		public: public,
	}
}

// KID - возвращает идентификатор ключа.
func (k ecdsaVerifyKey) KID() string { return k.kid }

// Method - возвращает алгоритм подписи (ES256).
func (k ecdsaVerifyKey) Method() jwt.SigningMethod { return jwt.SigningMethodES256 }

// Public - возвращает публичный ключ для проверки подписи.
func (k ecdsaVerifyKey) Public() any { return k.public }
