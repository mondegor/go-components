package validate

import (
	"regexp"
)

const (
	// nameLocal - имя часового пояса процесса. Оно проходит проверку формата,
	// но IANA-именем не является: его значение зависит от настроек хоста,
	// поэтому клиенту выбирать его не разрешается.
	//
	// Отдельная проверка здесь - подстраховка: список, с которым сверяется
	// wire/mrauth/config.TagTimeZone, ожидается от timezone.LocationList.TimeZones(),
	// а он "Local" наружу не отдаёт, да и сам список поясов приложения отбраковывается
	// на старте (go-core/util/timezone/config.ValidateTimeZones тоже отвергает "Local").
	// Проверка удерживает случай, когда host-приложение собрало список иначе.
	nameLocal = "Local"
)

// regexpTimeZone - формат IANA-имени часового пояса.
var regexpTimeZone = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+_-]*(/[A-Za-z0-9+_-]+){0,2}$`)

// TimeZone - сообщает о соответствии указанного значения формату
// IANA-имени часового пояса (напр. Europe/Moscow, UTC, America/Argentina/Salta).
//
// Проверяется только формат; зарегистрировано ли имя в приложении, здесь не проверяется -
// это отдельное условие, которое добавляется поверх (см. wire/mrauth/config.TagTimeZone):
// вход, сохраняемый в профиль, обязан точно совпадать с одним из поясов приложения,
// поэтому одной проверки формы для него недостаточно.
func TimeZone(value string) bool {
	return value != nameLocal && regexpTimeZone.MatchString(value)
}
