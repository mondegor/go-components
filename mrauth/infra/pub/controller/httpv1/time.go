package httpv1

import "time"

// formatTimeIn - форматирует абсолютный момент в часовом поясе loc в RFC3339
// (с точностью до секунды; дробная часть отбрасывается вниз, чтобы объявленный
// момент не оказался позже фактического). Время в домене хранится в UTC, поэтому
// здесь оно переводится в зону пользователя перед отдачей.
func formatTimeIn(tm time.Time, loc *time.Location) string {
	return tm.Truncate(time.Second).In(loc).Format(time.RFC3339)
}
