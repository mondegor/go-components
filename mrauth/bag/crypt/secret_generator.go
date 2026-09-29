package crypt

import (
	"errors"
	"fmt"

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
	// и аварийных кодов подтверждения. Длину секрета задаёт каждый вызов.
	SecretGenerator struct{}
)

// NewSecretGenerator - создаёт объект SecretGenerator.
func NewSecretGenerator() *SecretGenerator {
	return &SecretGenerator{}
}

// GenToken - генерирует случайный токен указанной длины.
func (c *SecretGenerator) GenToken(length int) (string, error) {
	token, err := crypt.GenerateToken(length)
	if err != nil {
		return "", fmt.Errorf("invalid GenToken: %w", err)
	}

	return token, nil
}

// GenCode - генерирует случайный цифровой код указанной длины.
func (c *SecretGenerator) GenCode(length int) (string, error) {
	code, err := crypt.GenerateDigits(length)
	if err != nil {
		return "", fmt.Errorf("invalid GenCode: %w", err)
	}

	return code, nil
}

// GenCodeWithHash - генерирует цифровой код подтверждения указанной длины и его bcrypt-хеш:
// хеш сохраняется в хранилище, открытый код отправляется пользователю.
func (c *SecretGenerator) GenCodeWithHash(length int) (code, hashedCode string, err error) {
	code, err = c.GenCode(length)
	if err != nil {
		return "", "", err
	}

	hashedCode, err = c.HashedSecret(code)
	if err != nil {
		return "", "", err
	}

	return code, hashedCode, nil
}

// GenRecoveryCode - генерирует аварийный код указанной длины из латиницы и цифр с разделителем посередине.
func (c *SecretGenerator) GenRecoveryCode(length int) (string, error) {
	code, err := crypt.GenerateBytes(charsetRecoveryCode, length)
	if err != nil {
		return "", fmt.Errorf("invalid GenRecoveryCode: %w", err)
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
		return "", fmt.Errorf("invalid HashedSecret: %w", err)
	}

	return string(hashed), nil
}

// CompareSecretAndHash - сверяет секрет с его bcrypt-хешем.
func (c *SecretGenerator) CompareSecretAndHash(secret, hashedSecret string) (ok bool, err error) {
	if err = bcrypt.CompareHashAndPassword([]byte(hashedSecret), []byte(secret)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}

		return false, fmt.Errorf("invalid CompareSecretAndHash: %w", err)
	}

	return true, nil
}

// GenerateRecoveryCodes - генерирует count одноразовых кодов указанной длины и их bcrypt-хеши.
func (c *SecretGenerator) GenerateRecoveryCodes(count, length int) (plain, hashed []string, err error) {
	// TODO: можно выделить один массив и разделить его на два
	plain = make([]string, 0, count)
	hashed = make([]string, 0, count)

	for i := 0; i < count; i++ {
		code, err := c.GenRecoveryCode(length)
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
