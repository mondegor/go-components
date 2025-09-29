package crypt

import (
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrlib/crypt"
)

type (
	// TokenGenerator - comment struct.
	TokenGenerator struct {
		defaultLength int
	}
)

// NewTokenGenerator - создаёт объект TokenGenerator.
func NewTokenGenerator(defaultLength int) *TokenGenerator {
	return &TokenGenerator{
		defaultLength: defaultLength,
	}
}

// GenToken - comments method.
func (t *TokenGenerator) GenToken() (string, error) {
	return t.genToken(t.defaultLength)
}

// GenTokenLen - comments method.
func (t *TokenGenerator) GenTokenLen(length int) (string, error) {
	if length < 1 {
		length = t.defaultLength
	}

	return t.genToken(length)
}

func (t *TokenGenerator) genToken(length int) (string, error) {
	token, err := crypt.GenerateToken(length)
	if err != nil {
		return "", mr.ErrInternal.Wrap(err, "details", "invalid GenToken")
	}

	return token, nil
}
