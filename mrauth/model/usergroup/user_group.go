// Package usergroup владеет форматом группы пользователя "{realm}/{kind}" - строки, которой
// mraccess.User обозначает принадлежность пользователя к realm'у и виду внутри него.
//
// Группа собирается на входе в систему (провайдеры пользователя) и разбирается на выходе
// (трассировка активности), поэтому склейка и разбор держатся здесь вместе: пока они парные,
// формат можно поменять в одном месте.
package usergroup

import (
	"fmt"
	"strings"
)

const (
	// separator - разделитель realm и вида пользователя внутри группы.
	separator = "/"
)

// Build - собирает группу пользователя из имени realm'а и имени вида пользователя внутри него.
//
// ОГРАНИЧЕНИЕ НА '/'. realm содержать разделитель может (например "site/admin"), kind - нет:
// Realm отрезает всё после ПОСЛЕДНЕГО разделителя. Оба имени задаются конфигом хоста
// и неизменны в рантайме, поэтому здесь не проверяются; kind проверяется один раз
// на старте хоста - ValidateKind, вызываемый из wire/mrauth/config.ValidateRealms.
func Build(realm, kind string) string {
	return realm + separator + kind
}

// ValidateKind - проверяет, что имя вида пользователя не содержит разделитель группы.
// Иначе Realm отрежет группу не по той границе, realm определится неверно, не найдётся
// в реестре, и активность пользователей этого вида пойдёт с сентинелом RealmID = 0:
// per-realm статистика потеряется (в лог уйдёт лишь одно сообщение в час). Вызывается
// на старте хоста (config.ValidateRealms), чтобы ошибка конфигурации проявлялась сразу.
func ValidateKind(kind string) error {
	if strings.Contains(kind, separator) {
		return fmt.Errorf("user kind name %q must not contain separator %q", kind, separator)
	}

	return nil
}

// Realm - извлекает имя realm'а из группы, собранной Build.
// Если разделителя нет, вся группа считается именем realm'а.
func Realm(group string) string {
	idx := strings.LastIndex(group, separator)
	if idx < 0 {
		return group
	}

	return group[:idx]
}
