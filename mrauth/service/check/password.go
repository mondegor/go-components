package check

import (
	"github.com/mondegor/go-sysmess/util/crypt/password"
)

type (
	// Password - сервис проверки надёжности и генерации паролей.
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

// CalcStrength - вычисляет уровень надёжности указанного пароля.
func (sv *Password) CalcStrength(userPassword string) (strength string) {
	return password.CalcStrength(userPassword).String()
}

// Generate - генерирует новый пароль заданной длины.
func (sv *Password) Generate() (strength string) {
	return password.NewGenerator().Generate(sv.length, password.CharAll) // TODO: в настройки
}
