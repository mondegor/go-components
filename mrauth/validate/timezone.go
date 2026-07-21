package validate

import (
	"regexp"
)

const (
	// nameLocal - имя часового пояса процесса. Оно проходит проверку формата,
	// но IANA-именем не является: его значение зависит от настроек хоста,
	// поэтому клиенту выбирать его не разрешается.
	// Пояс отбраковывается и на уровне подбора (mrauth/service/timezone),
	// т.к. в список поясов приложения он попадает всегда.
	nameLocal = "Local"
)

var regexpTimeZone = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+_-]*(/[A-Za-z0-9+_.-]+){0,2}$`)

// TimeZone - сообщает о соответствии указанного значения формату
// IANA-имени часового пояса (напр. Europe/Moscow, UTC, America/Argentina/Salta).
// Проверяется только формат: зарегистрировано ли имя в приложении,
// определяется подбором пояса на уровне usecase.
func TimeZone(value string) bool {
	return value != nameLocal && regexpTimeZone.MatchString(value)
}
