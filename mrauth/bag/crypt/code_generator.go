package crypt

import (
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/crypt"
	"golang.org/x/crypto/bcrypt"
)

type (
	// CodeGenerator - comment struct.
	CodeGenerator struct {
		defaultLength int
	}
)

// NewCodeGenerator - создаёт объект CodeGenerator.
func NewCodeGenerator(defaultLength int) *CodeGenerator {
	return &CodeGenerator{
		defaultLength: defaultLength,
	}
}

// GenCode - comments method.
func (c *CodeGenerator) GenCode() (string, error) {
	return c.genCodeLen(c.defaultLength)
}

// GenCodeLen - comments method.
func (c *CodeGenerator) GenCodeLen(length int) (string, error) {
	if length < 1 {
		length = c.defaultLength
	}

	return c.genCodeLen(length)
}

// HashedCode - comments method.
func (c *CodeGenerator) HashedCode(code string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.WrapInternalError(err, "invalid HashedCode")
	}

	return string(hashed), nil
}

// GenCodeAndHash - comments method.
func (c *CodeGenerator) GenCodeAndHash() (code, hashedCode string, err error) {
	code, err = c.GenCode()
	if err != nil {
		return "", "", err
	}

	hashedCode, err = c.HashedCode(code)
	if err != nil {
		return "", "", err
	}

	return code, hashedCode, nil
}

// CompareCodeAndHash - comments method.
func (c *CodeGenerator) CompareCodeAndHash(code, hashedCode string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(code))
}

func (c *CodeGenerator) genCodeLen(length int) (string, error) {
	code, err := crypt.GenerateDigits(length)
	if err != nil {
		return "", errors.WrapInternalError(err, "invalid GenCode")
	}

	return code, nil
}
