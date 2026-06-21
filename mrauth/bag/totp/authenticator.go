package totp

import (
	"image"
	"net/url"
	"time"

	"github.com/mondegor/go-sysmess/errors"
	pqotp "github.com/pquerna/otp"
	pqtotp "github.com/pquerna/otp/totp"
)

type (
	// Authenticator - адаптер над github.com/pquerna/otp, инкапсулирующий работу
	// с TOTP: генерацию секрета, проверку кода и построение QR-кода.
	Authenticator struct {
		issuer     string
		secretSize uint
	}
)

// NewAuthenticator - создаёт объект Authenticator.
func NewAuthenticator(issuer string, secretSize uint) *Authenticator {
	return &Authenticator{
		issuer:     issuer,
		secretSize: secretSize,
	}
}

// GenerateSecret - генерирует TOTP-секрет (base32) для указанного аккаунта.
func (a *Authenticator) GenerateSecret(accountName string) (string, error) {
	key, err := pqtotp.Generate(
		pqtotp.GenerateOpts{
			Issuer:      a.issuer,
			AccountName: accountName,
			SecretSize:  a.secretSize,
		},
	)
	if err != nil {
		return "", errors.WrapInternalError(err, "failed to generate TOTP secret")
	}

	return key.Secret(), nil
}

// Validate - проверяет TOTP-код против секрета (параметры по умолчанию: период 30с, 6 цифр, SHA1).
func (a *Authenticator) Validate(code, secret string) bool {
	return pqtotp.Validate(code, secret)
}

// GenerateCode - вычисляет TOTP-код для секрета на момент времени t.
func (a *Authenticator) GenerateCode(secret string, t time.Time) (string, error) {
	code, err := pqtotp.GenerateCode(secret, t)
	if err != nil {
		return "", errors.WrapInternalError(err, "failed to generate TOTP code")
	}

	return code, nil
}

// QRImage - строит QR-код TOTP-генератора для указанного аккаунта и секрета
// размером width x height.
func (a *Authenticator) QRImage(accountName, secret string, width, height int) (image.Image, error) {
	key, err := pqotp.NewKeyFromURL(a.otpauthURL(accountName, secret))
	if err != nil {
		return nil, errors.WrapInternalError(err, "failed to parse otpauth URL")
	}

	img, err := key.Image(width, height)
	if err != nil {
		return nil, errors.WrapInternalError(err, "failed to render TOTP QR image")
	}

	return img, nil
}

// otpauthURL - собирает otpauth-URL для построения QR из сохранённого секрета.
func (a *Authenticator) otpauthURL(account, secret string) string {
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", a.issuer)

	u := url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + a.issuer + ":" + account,
		RawQuery: query.Encode(),
	}

	return u.String()
}
