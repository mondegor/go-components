package check

import (
	"errors"

	"github.com/mondegor/go-core/util/crypt/password"
)

// maxGenerateAttempts - предельное число попыток сгенерировать пароль надёжности THE_BEST
// (одна попытка промахивается примерно в 4% случаев при длине 16).
const maxGenerateAttempts = 16

// errBestPasswordNotGenerated - за отведённое число попыток не удалось сгенерировать
// пароль надёжности THE_BEST (например, из-за длины меньше 12 символов).
var errBestPasswordNotGenerated = errors.New("failed to generate password with the best strength")

type (
	// Password - сервис оценки надёжности и генерации паролей; хранит порог надёжности
	// пароля 2FA, чтобы оценка для клиента и проверка при установке пароля совпадали.
	Password struct {
		length      int
		minStrength password.PassStrength
	}
)

// NewPassword - создаёт объект Password. Для надёжности THE_BEST длина
// генерируемого пароля должна быть не меньше 12 символов.
func NewPassword(length int, opts ...PasswordOption) *Password {
	o := newPasswordOptions(opts)

	return &Password{
		length:      length,
		minStrength: o.minStrength,
	}
}

// CalcStrength - вычисляет уровень надёжности указанного пароля и сообщает,
// проходит ли он порог надёжности пароля 2FA.
func (sv *Password) CalcStrength(userPassword string) (strength string, acceptable bool) {
	value := password.CalcStrength(userPassword)

	return value.String(), value >= sv.minStrength
}

// IsAcceptable - сообщает, проходит ли указанный пароль порог надёжности пароля 2FA.
func (sv *Password) IsAcceptable(userPassword string) bool {
	return password.CalcStrength(userPassword) >= sv.minStrength
}

// Generate - генерирует новый пароль заданной длины, надёжность которого всегда THE_BEST.
func (sv *Password) Generate() (userPassword string, err error) {
	generator := password.NewGenerator()

	for range maxGenerateAttempts {
		value := generator.Generate(sv.length, password.CharAll) // TODO: в настройки

		if password.CalcStrength(value) == password.PassStrengthBest {
			return value, nil
		}
	}

	return "", errBestPasswordNotGenerated
}
