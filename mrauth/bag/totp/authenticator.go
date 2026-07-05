package totp

import (
	"crypto/subtle"
	"image"
	"net/url"
	"time"

	"github.com/mondegor/go-core/errors"
	pqotp "github.com/pquerna/otp"
	pqtotp "github.com/pquerna/otp/totp"
)

const (
	totpPeriod = 30 // период действия TOTP-кода в секундах (RFC 6238).
	totpSkew   = 1  // допустимое отклонение в шагах (±1 шаг = ±30с).
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

// ValidateCode - проверяет TOTP-код против секрета с явными параметрами
// (период 30с, 6 цифр, SHA1, окно ±1 шаг) и сравнением за константное время.
// При совпадении возвращает номер time-step совпавшего кода (для защиты от replay).
func (a *Authenticator) ValidateCode(code, secret string) (ok bool, timeStep int64, err error) {
	opts := pqtotp.ValidateOpts{
		Period:    totpPeriod,
		Skew:      0,
		Digits:    pqotp.DigitsSix,
		Algorithm: pqotp.AlgorithmSHA1,
	}

	var candidateCode string

	currentStep := time.Now().Unix() / totpPeriod

	for delta := int64(-totpSkew); delta <= totpSkew; delta++ {
		candidateStep := currentStep + delta

		candidateCode, err = pqtotp.GenerateCodeCustom(secret, time.Unix(candidateStep*totpPeriod, 0), opts)
		if err != nil {
			return false, 0, errors.WrapInternalError(err, "failed to generate TOTP code")
		}

		if subtle.ConstantTimeCompare([]byte(candidateCode), []byte(code)) == 1 {
			return true, candidateStep, nil
		}
	}

	return false, 0, nil
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
	key, err := pqotp.NewKeyFromURL(a.otpAuthURL(accountName, secret))
	if err != nil {
		return nil, errors.WrapInternalError(err, "failed to parse otpauth URL")
	}

	img, err := key.Image(width, height)
	if err != nil {
		return nil, errors.WrapInternalError(err, "failed to render TOTP QR image")
	}

	return img, nil
}

// otpAuthURL - собирает otpauth-URL для построения QR из сохранённого секрета.
func (a *Authenticator) otpAuthURL(account, secret string) string {
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
