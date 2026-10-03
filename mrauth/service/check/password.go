package check

import (
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/util/crypt/password"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/enum/passwordacceptstatus"
)

// maxGenerateAttempts - предельное число попыток сгенерировать пароль надёжности THE_BEST
// (случайный пароль иногда не содержит всех наборов символов или нужного числа уникальных).
const maxGenerateAttempts = 8

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

// CalcStrength - вычисляет уровень надёжности указанного пароля и статус его приёма
// как пароля 2FA. Уровень надёжности от статуса приёма не зависит.
func (sv *Password) CalcStrength(userPassword string) (strength string, acceptStatus passwordacceptstatus.Enum) {
	value := password.CalcStrength(userPassword)

	return value.String(), sv.acceptStatus(userPassword, value)
}

// Check - проверяет, допустим ли указанный пароль как пароль 2FA. Недопустимый пароль
// отклоняется ошибкой причины отказа (mrauth.ErrPasswordIsTooWeak или
// mrauth.ErrPasswordHasRecoveryCodeFormat).
func (sv *Password) Check(userPassword string) error {
	switch sv.acceptStatus(userPassword, password.CalcStrength(userPassword)) {
	case passwordacceptstatus.Accepted:
		return nil
	case passwordacceptstatus.RecoveryCodeFormat:
		return mrauth.ErrPasswordHasRecoveryCodeFormat
	default:
		return mrauth.ErrPasswordIsTooWeak
	}
}

// acceptStatus - возвращает статус приёма пароля как пароля 2FA. Формат аварийного кода
// проверяется раньше порога надёжности: усиление пароля в том же формате его не исправит,
// а на звене пароля такой ввод неотличим от аварийного кода.
func (sv *Password) acceptStatus(userPassword string, strength password.PassStrength) passwordacceptstatus.Enum {
	if crypt.IsRecoveryCodeFormat(userPassword) {
		return passwordacceptstatus.RecoveryCodeFormat
	}

	if strength < sv.minStrength {
		return passwordacceptstatus.TooWeak
	}

	return passwordacceptstatus.Accepted
}

// Generate - генерирует новый пароль заданной длины, надёжность которого всегда THE_BEST.
func (sv *Password) Generate() (userPassword string, err error) {
	generator := password.NewGenerator()

	for range maxGenerateAttempts {
		value := generator.Generate(sv.length, password.CharAll) // TODO: в настройки
		strength := password.CalcStrength(value)

		if strength != password.PassStrengthBest {
			continue
		}

		if sv.acceptStatus(value, strength) == passwordacceptstatus.Accepted {
			return value, nil
		}
	}

	return "", errBestPasswordNotGenerated
}
