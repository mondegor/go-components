package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
)

func internalRSAPublicPEM(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func TestValidateJWT_RequiresKID(t *testing.T) {
	t.Parallel()

	base := JWT{
		Alg:    "HS256",
		Secret: "0123456789abcdef0123456789abcdef", // 32 байта, минимум для HS256
	}

	// без kid - ошибка
	require.Error(t, validateJWT(base))

	// с kid - ок
	base.KID = "k1"
	require.NoError(t, validateJWT(base))
}

func TestValidateJWT_HMACSecretMinLength(t *testing.T) {
	t.Parallel()

	base := func(alg, secret string) JWT {
		return JWT{
			Alg:    alg,
			KID:    "k1",
			Secret: secret,
		}
	}

	// HS256: < 32 байт - ошибка, >= 32 - ок
	require.Error(t, validateJWT(base("HS256", "short")))
	require.NoError(t, validateJWT(base("HS256", "0123456789abcdef0123456789abcdef")))

	// HS512: < 64 байт - ошибка
	require.Error(t, validateJWT(base("HS512", "0123456789abcdef0123456789abcdef")))
}

func TestValidateJWT_VerifyKeys(t *testing.T) {
	t.Parallel()

	base := func(keys ...JWTVerifyKey) JWT {
		return JWT{
			Alg:        "HS256",
			KID:        "active",
			Secret:     "0123456789abcdef0123456789abcdef",
			VerifyKeys: keys,
		}
	}

	publicPEM := internalRSAPublicPEM(t)

	// корректный verify-ключ
	require.NoError(t, validateJWT(base(JWTVerifyKey{KID: "old", Alg: "RS256", PublicKey: publicPEM})))

	// пустой kid - ошибка
	require.Error(t, validateJWT(base(JWTVerifyKey{KID: "", Alg: "RS256", PublicKey: publicPEM})))

	// kid дублирует активный ключ - ошибка (иначе ключ молча затирается в наборе)
	require.Error(t, validateJWT(base(JWTVerifyKey{KID: "active", Alg: "RS256", PublicKey: publicPEM})))

	// kid дублируется среди verify-ключей - ошибка
	require.Error(t, validateJWT(base(
		JWTVerifyKey{KID: "dup", Alg: "RS256", PublicKey: publicPEM},
		JWTVerifyKey{KID: "dup", Alg: "RS256", PublicKey: publicPEM},
	)))

	// неподдерживаемый алгоритм - ошибка
	require.Error(t, validateJWT(base(JWTVerifyKey{KID: "old", Alg: "HS256", PublicKey: publicPEM})))

	// пустой public_key - ошибка
	require.Error(t, validateJWT(base(JWTVerifyKey{KID: "old", Alg: "RS256", PublicKey: ""})))
}
