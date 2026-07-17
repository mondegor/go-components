package mrauth

import (
	"net/netip"

	"github.com/mondegor/go-core/mrtype"
)

const (
	// LocationOrEmpty - вернуть местоположение, если оно вычислено, иначе пустую строку.
	LocationOrEmpty LocationMode = iota

	// LocationOnlyIP - вернуть только IP адрес в виде строки (местоположение не вычисляется).
	LocationOnlyIP

	// LocationOrIP - вернуть местоположение, если оно вычислено, иначе IP адрес в виде строки.
	LocationOrIP
)

type (
	// LocationMode - режим формирования результата LocationResolver (см. константы Location*).
	LocationMode uint8

	// LocationResolver - определяет местоположение по IP адресу; параметр result задаёт,
	// что вернуть, когда местоположение вычислено, а что - когда нет (см. константы Location*).
	// Адрес нормально всегда задан: источник - RemoteAddr, а колонки, из которых он читается
	// (last_login_ip, sessions.last_ip), объявлены NOT NULL. Тем не менее реализация должна выдержать
	// и незаданный netip.Addr - DefaultLocationResolver в этом случае возвращает пустую строку.
	LocationResolver func(ip netip.Addr, mode LocationMode) string

	// AppResolver - определяет приложение и устройство по строке User-Agent.
	// Вход недоверенный (контролируется клиентом) - его нельзя писать в логи без
	// экранирования (CRLF/log-forging) и нельзя слепо подставлять во внешние запросы.
	AppResolver func(userAgent string) (appName, deviceName string)
)

// DefaultLocationResolver - резолвер, используемый когда хост не задал свой.
// Просто отдаёт исходный IP, для не заданного адреса - пустая строка.
func DefaultLocationResolver(ip netip.Addr, mode LocationMode) string {
	if mode == LocationOnlyIP || mode == LocationOrIP {
		return mrtype.NewIP(ip).String()
	}

	return ""
}
