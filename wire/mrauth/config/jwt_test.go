package config_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/wire/mrauth/config"
)

func rsaPublicPEM(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func rsaPrivatePEM(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}

	return string(pem.EncodeToMemory(block))
}

func ecdsaPrivatePEM(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
}

func TestNewJWT(t *testing.T) {
	t.Parallel()

	t.Run("HS256", func(t *testing.T) {
		t.Parallel()

		got, err := config.InitJWT(config.JWT{Alg: "HS256", KID: "k1", Secret: "0123456789abcdef0123456789abcdef"})
		require.NoError(t, err)
		require.NotNil(t, got.SigningKey)
		assert.Equal(t, "HS256", got.SigningKey.Method().Alg())
		assert.Equal(t, "k1", got.SigningKey.KID())
		assert.NotNil(t, got.Verifier)
	})

	t.Run("HS512", func(t *testing.T) {
		t.Parallel()

		got, err := config.InitJWT(config.JWT{Alg: "HS512", KID: "k1", Secret: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"})
		require.NoError(t, err)
		assert.Equal(t, "HS512", got.SigningKey.Method().Alg())
	})

	t.Run("RS256 from PEM", func(t *testing.T) {
		t.Parallel()

		got, err := config.InitJWT(config.JWT{Alg: "RS256", KID: "rsa-1", Secret: rsaPrivatePEM(t)})
		require.NoError(t, err)
		require.NotNil(t, got.SigningKey)
		assert.Equal(t, "RS256", got.SigningKey.Method().Alg())
	})

	t.Run("ES256 from PEM", func(t *testing.T) {
		t.Parallel()

		got, err := config.InitJWT(config.JWT{Alg: "ES256", KID: "es-1", Secret: ecdsaPrivatePEM(t)})
		require.NoError(t, err)
		assert.Equal(t, "ES256", got.SigningKey.Method().Alg())
	})

	t.Run("RS256 with broken PEM - error", func(t *testing.T) {
		t.Parallel()

		_, err := config.InitJWT(config.JWT{Alg: "RS256", Secret: "not-a-pem"})
		require.Error(t, err)
	})

	t.Run("with verify-only keys", func(t *testing.T) {
		t.Parallel()

		got, err := config.InitJWT(config.JWT{
			Alg:    "RS256",
			KID:    "new",
			Secret: rsaPrivatePEM(t),
			VerifyKeys: []config.JWTVerifyKey{
				{KID: "old", Alg: "RS256", PublicKey: rsaPublicPEM(t)},
			},
		})
		require.NoError(t, err)

		body, err := got.Verifier.JWKS()
		require.NoError(t, err)

		var set struct {
			Keys []map[string]any `json:"keys"`
		}
		require.NoError(t, json.Unmarshal(body, &set))
		assert.Len(t, set.Keys, 2) // активный signing + verify-only
	})

	t.Run("broken verify key PEM - error", func(t *testing.T) {
		t.Parallel()

		_, err := config.InitJWT(config.JWT{
			Alg:        "RS256",
			KID:        "new",
			Secret:     rsaPrivatePEM(t),
			VerifyKeys: []config.JWTVerifyKey{{KID: "old", Alg: "RS256", PublicKey: "not-a-pem"}},
		})
		require.Error(t, err)
	})

	t.Run("unsupported alg - error", func(t *testing.T) {
		t.Parallel()

		_, err := config.InitJWT(config.JWT{Alg: "ZZ999", Secret: "secret"})
		require.Error(t, err)
	})

	t.Run("without secret - error", func(t *testing.T) {
		t.Parallel()

		// cfg.Secret обязателен: без ключевого материала verifier был бы nil и утёк бы
		// в JWKS/parser, приведя к panic в рантайме
		_, err := config.InitJWT(config.JWT{
			Alg: "HS256",
			KID: "k1",
		})
		require.Error(t, err)
	})
}

func TestIsJWTUsed(t *testing.T) {
	t.Parallel()

	jwtRealm := config.UserRealm{AuthToken: config.Token{AccessType: "jwt"}}
	sessionRealm := config.UserRealm{AuthToken: config.Token{AccessType: "session"}}

	assert.True(t, config.IsJWTUsed([]config.UserRealm{sessionRealm, jwtRealm}))
	assert.False(t, config.IsJWTUsed([]config.UserRealm{sessionRealm}))
	assert.False(t, config.IsJWTUsed(nil))
}

// TestValidateJWT_* (прямые тесты приватной validateJWT) находятся
// в internal-файле validate_jwt_internal_test.go (package config).
