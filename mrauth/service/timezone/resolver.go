package timezone

import (
	"time"

	"github.com/mondegor/go-components/mrauth/dto"
)

const (
	// defaultTimeZone - пояс, который возвращается, если подобрать пояс не удалось.
	// Зарегистрирован в списке поясов всегда.
	defaultTimeZone = "UTC"

	// nameLocal - имя часового пояса процесса. Оно зарегистрировано в списке поясов,
	// но IANA-именем не является: его значение зависит от настроек хоста,
	// поэтому подбирать его пользователю не разрешается.
	// То же имя отбраковывается проверкой формата (mrauth/validate.TimeZone).
	nameLocal = "Local"
)

type (
	// Resolver - подбирает часовой пояс, зарегистрированный в приложении,
	// по значениям, присланным пользователем.
	Resolver struct {
		locations locationList
	}

	// locationList - предоставляет доступ к предзагруженным часовым поясам приложения.
	// Если имя пояса в списке не зарегистрировано, то LocationByName возвращает пояс
	// по умолчанию и ошибку.
	locationList interface {
		LocationByName(value string) (*time.Location, error)
		NameByOffset(offset time.Duration, isDST bool) (name string, ok bool)
	}
)

// New - создаёт объект Resolver.
func New(locations locationList) *Resolver {
	return &Resolver{
		locations: locations,
	}
}

// Resolve - подбирает пояс, зарегистрированный в приложении: сначала по имени,
// затем по смещению относительно UTC и признаку летнего времени, а если не подошло
// ни то, ни другое - возвращается пояс по умолчанию.
//
// Имя, присланное клиентом, может быть корректным, но приложению неизвестным: список
// поясов ограничен настройками приложения, а база часовых поясов у клиента бывает новее
// серверной. Поэтому такое имя не отвергается, а заменяется на эквивалентный по смещению
// пояс из списка.
//
// Результат подбора по смещению от момента вызова не зависит: индекс поясов строится
// один раз при создании списка сразу по всем сезонным состояниям (go-core/util/timezone).
// От момента зависит сама пара (смещение, признак летнего времени): она описывает
// состояние пояса у клиента на момент замера, поэтому подбирать по ней следует пояс,
// а не хранить её саму.
func (sv *Resolver) Resolve(in dto.TimeZoneInfo) string {
	// пояс процесса пользователю не принадлежит: он зарегистрирован в списке,
	// поэтому отбраковывается до обращения к нему по имени
	if in.Name != nameLocal {
		if _, err := sv.locations.LocationByName(in.Name); err == nil {
			return in.Name
		}
	}

	if name, ok := sv.locations.NameByOffset(in.Offset, in.IsDST); ok {
		return name
	}

	return defaultTimeZone
}
