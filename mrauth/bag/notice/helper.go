package notice

import "strings"

// KeyByEventAndRealm - формирует ключ события уведомления, добавляя к базовому событию realm;
// символы "/" в realm заменяются на "." для сохранения ключа в виде точечного пространства имён.
func KeyByEventAndRealm(event, realm string) string {
	return event + "." + strings.ReplaceAll(realm, "/", ".")
}
