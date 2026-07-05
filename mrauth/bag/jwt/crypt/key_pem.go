package crypt

import (
	"crypto/elliptic"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// NewRSAKeyFromPEM - создаёт RSA-ключ подписи (RS256) из приватного ключа в формате PEM.
func NewRSAKeyFromPEM(kid string, privatePEM []byte) (SigningKey, error) {
	private, err := jwt.ParseRSAPrivateKeyFromPEM(privatePEM)
	if err != nil {
		return nil, err
	}

	return NewRSAKey(kid, private), nil
}

// NewECDSAKeyFromPEM - создаёт ECDSA-ключ подписи (ES256) из приватного ключа в формате PEM.
func NewECDSAKeyFromPEM(kid string, privatePEM []byte) (SigningKey, error) {
	private, err := jwt.ParseECPrivateKeyFromPEM(privatePEM)
	if err != nil {
		return nil, err
	}

	if err = ensureP256(private.Curve); err != nil {
		return nil, err
	}

	return NewECDSAKey(kid, private), nil
}

// NewRSAVerifyKeyFromPEM - создаёт RSA-ключ только для проверки (RS256) из публичного ключа в формате PEM.
func NewRSAVerifyKeyFromPEM(kid string, publicPEM []byte) (Key, error) {
	public, err := jwt.ParseRSAPublicKeyFromPEM(publicPEM)
	if err != nil {
		return nil, err
	}

	return NewRSAVerifyKey(kid, public), nil
}

// NewECDSAVerifyKeyFromPEM - создаёт ECDSA-ключ только для проверки (ES256) из публичного ключа в формате PEM.
func NewECDSAVerifyKeyFromPEM(kid string, publicPEM []byte) (Key, error) {
	public, err := jwt.ParseECPublicKeyFromPEM(publicPEM)
	if err != nil {
		return nil, err
	}

	if err = ensureP256(public.Curve); err != nil {
		return nil, err
	}

	return NewECDSAVerifyKey(kid, public), nil
}

// ensureP256 - ES256 (RFC 7518 §3.4) работает только с кривой P-256; иначе подпись/проверка
// упадёт в рантайме, поэтому отбраковываем ключ ещё при загрузке.
func ensureP256(curve elliptic.Curve) error {
	if curve != elliptic.P256() {
		return fmt.Errorf("ECDSA curve %q is not supported: ES256 requires P-256", curve.Params().Name)
	}

	return nil
}
