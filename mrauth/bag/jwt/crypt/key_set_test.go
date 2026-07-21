package crypt_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/jwt"
	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
)

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	return key
}

func mustECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return key
}

// mustHMACKey - создаёт HMAC-ключ, прерывая тест при неподдерживаемом методе.
func mustHMACKey(t *testing.T, kid, method string, secret []byte) crypt.SigningKey {
	t.Helper()

	key, err := crypt.NewHMACKey(kid, method, secret)
	require.NoError(t, err)

	return key
}

// mustKeySet - собирает набор ключей, прерывая тест при ошибке (например, дубликат kid).
func mustKeySet(t *testing.T, keys ...crypt.Key) crypt.KeySet {
	t.Helper()

	keySet, err := crypt.NewKeySet(keys...)
	require.NoError(t, err)

	return keySet
}

// validClaims - возвращает набор claim'ов корректного access токена.
func validClaims() gojwt.MapClaims {
	return gojwt.MapClaims{
		"aud":   "site/admin",
		"sub":   uuid.New().String(),
		"sid":   "523266583",
		"lan":   "en",
		"tz":    "Europe/Moscow",
		"scope": "admin",
		"exp":   gojwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
	}
}

// signWith - подписывает корректные claim'ы указанным методом и приватным ключом,
// проставляя заголовок 'kid' (если задан).
func signWith(t *testing.T, method gojwt.SigningMethod, key any, kid string) string {
	t.Helper()

	token := gojwt.NewWithClaims(method, validClaims())
	if kid != "" {
		token.Header["kid"] = kid
	}

	signed, err := token.SignedString(key)
	require.NoError(t, err)

	return signed
}

func TestParser_Parse_RS256(t *testing.T) {
	t.Parallel()

	key := mustRSAKey(t)
	token := signWith(t, gojwt.SigningMethodRS256, key, "rsa-1")

	keySet := mustKeySet(t, crypt.NewRSAKey("rsa-1", key))

	got, err := jwt.NewParser(keySet).Parse(token)
	require.NoError(t, err)
	assert.Equal(t, "site/admin", got.Realm)
	assert.Equal(t, uint32(523266583), got.SessionID)
}

func TestParser_Parse_ES256(t *testing.T) {
	t.Parallel()

	key := mustECDSAKey(t)
	token := signWith(t, gojwt.SigningMethodES256, key, "ec-1")

	keySet := mustKeySet(t, crypt.NewECDSAKey("ec-1", key))

	got, err := jwt.NewParser(keySet).Parse(token)
	require.NoError(t, err)
	assert.Equal(t, "site/admin", got.Realm)
}

func TestParser_Parse_AlgNoneRejected(t *testing.T) {
	t.Parallel()

	token := signWith(t, gojwt.SigningMethodNone, gojwt.UnsafeAllowNoneSignatureType, "")

	keySet := mustKeySet(t, crypt.NewRSAKey("rsa-1", mustRSAKey(t)))

	_, err := jwt.NewParser(keySet).Parse(token)
	require.ErrorIs(t, err, jwt.ErrTokenInvalid)
}

func TestParser_Parse_UnknownKID(t *testing.T) {
	t.Parallel()

	key := mustRSAKey(t)
	token := signWith(t, gojwt.SigningMethodRS256, key, "other-kid")

	keySet := mustKeySet(t, crypt.NewRSAKey("rsa-1", key))

	_, err := jwt.NewParser(keySet).Parse(token)
	require.ErrorIs(t, err, jwt.ErrTokenInvalid)
}

func TestParser_Parse_SelectsByKID(t *testing.T) {
	t.Parallel()

	rsa1 := mustRSAKey(t)
	rsa2 := mustRSAKey(t)

	keySet := mustKeySet(t,
		crypt.NewRSAKey("rsa-1", rsa1),
		crypt.NewRSAKey("rsa-2", rsa2),
	)

	// токен подписан вторым ключом и помечен его kid - должен проверяться именно им
	token := signWith(t, gojwt.SigningMethodRS256, rsa2, "rsa-2")

	got, err := jwt.NewParser(keySet).Parse(token)
	require.NoError(t, err)
	assert.Equal(t, "site/admin", got.Realm)

	// тот же токен, но с kid первого ключа - подпись не сойдётся
	tokenWrongKID := signWith(t, gojwt.SigningMethodRS256, rsa2, "rsa-1")

	_, err = jwt.NewParser(keySet).Parse(tokenWrongKID)
	require.ErrorIs(t, err, jwt.ErrTokenInvalid)
}

func TestParser_Parse_RotationWithVerifyOnlyKey(t *testing.T) {
	t.Parallel()

	oldKey := mustRSAKey(t)
	newKey := mustRSAKey(t)

	// набор: новый активный ключ (signing) + старый только для проверки (verify-only)
	keySet := mustKeySet(t,
		crypt.NewRSAKey("new", newKey),
		crypt.NewRSAVerifyKey("old", &oldKey.PublicKey),
	)

	// токен, выпущенный СТАРЫМ ключом (kid=old), должен оставаться валидным в период ротации
	oldToken := signWith(t, gojwt.SigningMethodRS256, oldKey, "old")
	_, err := jwt.NewParser(keySet).Parse(oldToken)
	require.NoError(t, err)

	// токен новым ключом (kid=new) тоже валиден
	newToken := signWith(t, gojwt.SigningMethodRS256, newKey, "new")
	_, err = jwt.NewParser(keySet).Parse(newToken)
	require.NoError(t, err)
}

func TestParser_Parse_AlgConfusionRejected(t *testing.T) {
	t.Parallel()

	rsaPrivate := mustRSAKey(t)
	keySet := mustKeySet(t, crypt.NewRSAKey("rsa-1", rsaPrivate))

	// классическая HS/RS confusion: подписать HS256, используя как секрет публичный RSA-ключ;
	// type-based пин алгоритма должен это отклонить (HMAC-метод против RSA-ключа)
	token := signWith(t, gojwt.SigningMethodHS256, []byte("public-key-as-hmac-secret"), "rsa-1")

	_, err := jwt.NewParser(keySet).Parse(token)
	require.ErrorIs(t, err, jwt.ErrTokenInvalid)
}

func TestKeySet_JWKS(t *testing.T) {
	t.Parallel()

	keySet := mustKeySet(t,
		crypt.NewRSAKey("rsa-1", mustRSAKey(t)),
		crypt.NewECDSAKey("ec-1", mustECDSAKey(t)),
		mustHMACKey(t, "hmac-1", "HS256", []byte("secret")),
	)

	body, err := keySet.JWKS()
	require.NoError(t, err)

	var set struct {
		Keys []map[string]any `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(body, &set))

	// HMAC-ключ (симметричный) не экспортируется - остаются только RSA и EC
	require.Len(t, set.Keys, 2)

	byKID := make(map[string]map[string]any, len(set.Keys))
	for _, key := range set.Keys {
		byKID[key["kid"].(string)] = key
	}

	require.Contains(t, byKID, "rsa-1")
	assert.Equal(t, "RSA", byKID["rsa-1"]["kty"])
	assert.Equal(t, "RS256", byKID["rsa-1"]["alg"])
	assert.Equal(t, "sig", byKID["rsa-1"]["use"])
	assert.NotEmpty(t, byKID["rsa-1"]["n"])
	assert.NotEmpty(t, byKID["rsa-1"]["e"])

	require.Contains(t, byKID, "ec-1")
	assert.Equal(t, "EC", byKID["ec-1"]["kty"])
	assert.Equal(t, "P-256", byKID["ec-1"]["crv"])
	assert.NotEmpty(t, byKID["ec-1"]["x"])
	assert.NotEmpty(t, byKID["ec-1"]["y"])

	assert.NotContains(t, byKID, "hmac-1")
}

func TestNewHMACKey_Method(t *testing.T) {
	t.Parallel()

	t.Run("empty method defaults to HS256", func(t *testing.T) {
		t.Parallel()

		key, err := crypt.NewHMACKey("k1", "", []byte("secret"))
		require.NoError(t, err)
		assert.Equal(t, "HS256", key.Method().Alg())
	})

	t.Run("HS512", func(t *testing.T) {
		t.Parallel()

		key, err := crypt.NewHMACKey("k1", "HS512", []byte("secret"))
		require.NoError(t, err)
		assert.Equal(t, "HS512", key.Method().Alg())
	})

	t.Run("unsupported method - error, no silent downgrade", func(t *testing.T) {
		t.Parallel()

		_, err := crypt.NewHMACKey("k1", "HS999", []byte("secret"))
		require.Error(t, err)
	})
}
