package crypt

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/golang-jwt/jwt/v5"
)

const (
	nonameKeyID = "noname"
)

type (
	// KeySet - набор ключей, индексированных по 'kid', для проверки подписи токенов.
	KeySet interface {
		KeyByKID(kid string) (Key, bool)
		JWKS() ([]byte, error)
	}

	// Key - ключ проверки подписи access-токена.
	// Для HMAC Public возвращает общий секрет, для асимметричных алгоритмов - публичный ключ.
	Key interface {
		KID() string               // идентификатор ключа (заголовок 'kid'); пустой - ключ без идентификатора
		Method() jwt.SigningMethod // алгоритм подписи
		Public() any               // ключ для проверки подписи
	}

	// SigningKey - ключ подписи access-токена (issuer-сторона): помимо проверки умеет подписывать.
	SigningKey interface {
		Key

		Private() any // ключ для подписи (token.SignedString)
	}

	keySet struct {
		byKID map[string]Key
	}

	jwkSet struct {
		Keys []jwk `json:"keys"`
	}

	// jwk - публичный ключ в формате JWK (RFC 7517).
	jwk struct {
		Kty string `json:"kty"`
		Use string `json:"use"`
		Kid string `json:"kid,omitempty"`
		Alg string `json:"alg,omitempty"`
		N   string `json:"n,omitempty"`   // RSA: модуль
		E   string `json:"e,omitempty"`   // RSA: публичная экспонента
		Crv string `json:"crv,omitempty"` // EC: кривая
		X   string `json:"x,omitempty"`   // EC: координата X
		Y   string `json:"y,omitempty"`   // EC: координата Y
	}
)

// NewKeySet - создаёт набор ключей для проверки подписи.
func NewKeySet(keys ...Key) (KeySet, error) {
	byKID := make(map[string]Key, len(keys))
	for _, key := range keys {
		keyID := key.KID()
		if keyID == "" {
			keyID = nonameKeyID
		}

		if _, ok := byKID[keyID]; ok {
			return nil, fmt.Errorf("duplicate jwt crypt key ID %q", keyID)
		}

		byKID[keyID] = key
	}

	return &keySet{
		byKID: byKID,
	}, nil
}

// KeyByKID - возвращает ключ по идентификатору.
func (ks *keySet) KeyByKID(kid string) (Key, bool) {
	if kid == "" {
		kid = nonameKeyID
	}

	key, ok := ks.byKID[kid]

	return key, ok
}

// JWKS - формирует тело JWKS (RFC 7517) с публичными ключами набора для отдачи на
// /.well-known/jwks.json. Симметричные (HMAC) ключи не экспортируются.
func (ks *keySet) JWKS() ([]byte, error) {
	set := jwkSet{Keys: make([]jwk, 0, len(ks.byKID))}

	for _, key := range ks.byKID {
		value, ok := publicJWK(key)
		if !ok {
			continue // HMAC и неизвестные типы не экспортируются
		}

		set.Keys = append(set.Keys, value)
	}

	return json.Marshal(set)
}

// publicJWK - строит JWK по публичному ключу; ok=false для неэкспортируемых (HMAC) ключей.
func publicJWK(key Key) (jwk, bool) {
	switch public := key.Public().(type) {
	case *rsa.PublicKey:
		return jwk{
			Kty: "RSA",
			Use: "sig",
			Kid: key.KID(),
			Alg: key.Method().Alg(),
			N:   base64.RawURLEncoding.EncodeToString(public.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(public.E)).Bytes()),
		}, true
	case *ecdsa.PublicKey:
		size := (public.Curve.Params().BitSize + 7) / 8

		return jwk{
			Kty: "EC",
			Use: "sig",
			Kid: key.KID(),
			Alg: key.Method().Alg(),
			Crv: public.Curve.Params().Name,
			X:   base64.RawURLEncoding.EncodeToString(public.X.FillBytes(make([]byte, size))),
			Y:   base64.RawURLEncoding.EncodeToString(public.Y.FillBytes(make([]byte, size))),
		}, true
	default:
		return jwk{}, false
	}
}
