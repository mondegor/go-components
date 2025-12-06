package check

import (
	"github.com/mondegor/go-sysmess/mrlib/crypt/password"
)

type (
	// Password - comment struct.
	Password struct {
		length int
	}
)

// NewPassword - создаёт объект Password.
func NewPassword(length int) *Password {
	return &Password{
		length: length,
	}
}

// CalcStrength - comments method.
func (s *Password) CalcStrength(userPassword string) (strength string) {
	return password.CalcStrength(userPassword).String()
}

// Generate - comments method.
func (s *Password) Generate() (strength string) {
	return password.NewGenerator().Generate(s.length, password.CharAll) // TODO: в настройки
}
