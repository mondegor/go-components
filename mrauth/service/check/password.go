package check

import (
	"errors"

	"github.com/mondegor/go-core/util/crypt/password"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
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
// допустим ли он как пароль 2FA (см. IsAcceptable).
func (sv *Password) CalcStrength(userPassword string) (strength string, acceptable bool) {
	value := password.CalcStrength(userPassword)

	return value.String(), sv.isAcceptable(userPassword, value)
}

// IsAcceptable - сообщает, допустим ли указанный пароль как пароль 2FA: он проходит порог
// надёжности и не имеет формата аварийного кода. Иначе на звене пароля такой ввод
// неотличим от аварийного кода, который там не принимается.
func (sv *Password) IsAcceptable(userPassword string) bool {
	return sv.isAcceptable(userPassword, password.CalcStrength(userPassword))
}

func (sv *Password) isAcceptable(userPassword string, strength password.PassStrength) bool {
	return strength >= sv.minStrength && !crypt.IsRecoveryCodeFormat(userPassword)
}

// Generate - генерирует новый пароль заданной длины, надёжность которого всегда THE_BEST.
func (sv *Password) Generate() (userPassword string, err error) {
	generator := password.NewGenerator()

	for range maxGenerateAttempts {
		value := generator.Generate(sv.length, password.CharAll) // TODO: в настройки
		strength := password.CalcStrength(value)

		if strength == password.PassStrengthBest && sv.isAcceptable(value, strength) {
			return value, nil
		}
	}

	return "", errBestPasswordNotGenerated
}
