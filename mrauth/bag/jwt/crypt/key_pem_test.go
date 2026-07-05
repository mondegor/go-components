package crypt_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
)

func ecPrivatePEM(t *testing.T, curve elliptic.Curve) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
}

func ecPublicPEM(t *testing.T, curve elliptic.Curve) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

func TestNewECDSAKeyFromPEM_CurvePinned(t *testing.T) {
	t.Parallel()

	t.Run("P-256 signing key - ok", func(t *testing.T) {
		t.Parallel()

		key, err := crypt.NewECDSAKeyFromPEM("k1", ecPrivatePEM(t, elliptic.P256()))
		require.NoError(t, err)
		assert.Equal(t, "ES256", key.Method().Alg())
	})

	t.Run("P-384 signing key - rejected", func(t *testing.T) {
		t.Parallel()

		_, err := crypt.NewECDSAKeyFromPEM("k1", ecPrivatePEM(t, elliptic.P384()))
		require.Error(t, err)
	})

	t.Run("P-256 verify key - ok", func(t *testing.T) {
		t.Parallel()

		_, err := crypt.NewECDSAVerifyKeyFromPEM("k1", ecPublicPEM(t, elliptic.P256()))
		require.NoError(t, err)
	})

	t.Run("P-521 verify key - rejected", func(t *testing.T) {
		t.Parallel()

		_, err := crypt.NewECDSAVerifyKeyFromPEM("k1", ecPublicPEM(t, elliptic.P521()))
		require.Error(t, err)
	})
}
