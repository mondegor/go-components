package totp_test

import (
	"testing"
	"time"

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
