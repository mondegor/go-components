package totp_test

import (
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/totp"
)

func TestAuthenticator_GenerateSecretRoundTrip(t *testing.T) {
	t.Parallel()

	auth := totp.NewAuthenticator("TestIssuer", 64)

	secret, err := auth.GenerateSecret("user@example.com")
	require.NoError(t, err)
	require.NotEmpty(t, secret)

	// secret валиден: код, вычисленный по нему, проходит проверку и возвращает номер шага
	code, err := auth.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	ok, step, err := auth.ValidateCode(code, secret)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotZero(t, step)
}

func TestAuthenticator_ValidateCodeRejectsWrongCode(t *testing.T) {
	t.Parallel()

	auth := totp.NewAuthenticator("TestIssuer", 64)

	secret, err := auth.GenerateSecret("user@example.com")
	require.NoError(t, err)

	ok, step, err := auth.ValidateCode("000000", secret)
	require.NoError(t, err)
	require.False(t, ok)
	require.Zero(t, step)
}

func TestAuthenticator_QRImage(t *testing.T) {
	t.Parallel()

	auth := totp.NewAuthenticator("TestIssuer", 64)

	secret, err := auth.GenerateSecret("user@example.com")
	require.NoError(t, err)

	img, err := auth.QRImage("user@example.com", secret, 256, 256)
	require.NoError(t, err)
	require.NotNil(t, img)
	require.Equal(t, 256, img.Bounds().Dx())
	require.Equal(t, 256, img.Bounds().Dy())
}

// TestAuthenticator_OTPAuthURL - ссылка отдаётся клиенту как есть (и строкой, и в QR-коде),
// поэтому её форма фиксируется целиком: схема otpauth://totp, метка "issuer:account"
// и параметры secret/issuer. Параметры генератора в неё намеренно не пишутся - используемые
// значения совпадают с дефолтными для otpauth.
func TestAuthenticator_OTPAuthURL(t *testing.T) {
	t.Parallel()

	auth := totp.NewAuthenticator("TestIssuer", 64)

	require.Equal(
		t,
		"otpauth://totp/TestIssuer:user@example.com?issuer=TestIssuer&secret=JBSWY3DPEHPK3PXP",
		auth.OTPAuthURL("user@example.com", "JBSWY3DPEHPK3PXP"),
	)
}

// TestAuthenticator_OTPAuthURLIsParsable - ссылка должна разбираться той же библиотекой,
// что строит по ней QR-код: иначе клиент получил бы валидный на вид URI, по которому
// QR-ручка отвечает ошибкой.
func TestAuthenticator_OTPAuthURLIsParsable(t *testing.T) {
	t.Parallel()

	auth := totp.NewAuthenticator("TestIssuer", 64)

	secret, err := auth.GenerateSecret("user@example.com")
	require.NoError(t, err)

	key, err := otp.NewKeyFromURL(auth.OTPAuthURL("user@example.com", secret))
	require.NoError(t, err)
	require.Equal(t, secret, key.Secret())
	require.Equal(t, "TestIssuer", key.Issuer())
	require.Equal(t, "user@example.com", key.AccountName())
}
