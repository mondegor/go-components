package crypt

import (
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/util/crypt"
	"golang.org/x/crypto/bcrypt"
)

const (
	minRecoveryCodeLengthWithSeparator = 11
)

//nolint:gochecknoglobals
var (
	charsetRecoveryCode = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
)

type (
	// SecretGenerator - генератор и хешировщик секретов: токенов, цифровых
	// и аварийных кодов подтверждения.
	SecretGenerator struct {
		secretLength int
	}
)

// NewSecretGenerator - создаёт объект SecretGenerator.
func NewSecretGenerator(defaultLength int) *SecretGenerator {
	return &SecretGenerator{
		secretLength: defaultLength,
	}
}

// GenToken - генерирует случайный токен заданной длины.
func (c *SecretGenerator) GenToken() (string, error) {
	token, err := crypt.GenerateToken(c.secretLength)
	if err != nil {
		return "", errors.WrapInternalError(err, "invalid GenToken")
	}

	return token, nil
}

// GenCode - генерирует случайный цифровой код заданной длины.
func (c *SecretGenerator) GenCode() (string, error) {
	code, err := crypt.GenerateDigits(c.secretLength)
	if err != nil {
		return "", errors.WrapInternalError(err, "invalid GenCode")
	}

	return code, nil
}

// GenCodeWithHash - генерирует цифровой код подтверждения и его bcrypt-хеш:
// хеш сохраняется в хранилище, открытый код отправляется пользователю.
func (c *SecretGenerator) GenCodeWithHash() (code, hashedCode string, err error) {
	code, err = c.GenCode()
	if err != nil {
		return "", "", err
	}

	hashedCode, err = c.HashedSecret(code)
	if err != nil {
		return "", "", err
	}

	return code, hashedCode, nil
}

// GenRecoveryCode - генерирует аварийный код из латиницы и цифр с разделителем посередине.
func (c *SecretGenerator) GenRecoveryCode() (string, error) {
	code, err := crypt.GenerateBytes(charsetRecoveryCode, c.secretLength)
	if err != nil {
		return "", errors.WrapInternalError(err, "invalid GenRecoveryCode")
	}

	if len(code) >= minRecoveryCodeLengthWithSeparator {
		code[len(code)/2] = '-'
	}

	return string(code), nil
}

// HashedSecret - возвращает bcrypt-хеш переданного секрета.
func (c *SecretGenerator) HashedSecret(value string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.WrapInternalError(err, "invalid HashedSecret")
	}

	return string(hashed), nil
}

// CompareSecretAndHash - сверяет секрет с его bcrypt-хешем.
func (c *SecretGenerator) CompareSecretAndHash(secret, hashedSecret string) (ok bool, err error) {
	if err = bcrypt.CompareHashAndPassword([]byte(hashedSecret), []byte(secret)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}

		return false, errors.WrapInternalError(err, "invalid CompareSecretAndHash")
	}

	return true, nil
}

// GenerateRecoveryCodes - генерирует count одноразовых кодов и их bcrypt-хеши.
func (c *SecretGenerator) GenerateRecoveryCodes(count int) (plain, hashed []string, err error) {
	// TODO: можно выделить один массив и разделить его на два
	plain = make([]string, 0, count)
	hashed = make([]string, 0, count)

	for i := 0; i < count; i++ {
		code, err := c.GenRecoveryCode()
		if err != nil {
			return nil, nil, err
		}

		hash, err := c.HashedSecret(code)
		if err != nil {
			return nil, nil, err
		}

		plain = append(plain, code)
		hashed = append(hashed, hash)
	}

	return plain, hashed, nil
}
