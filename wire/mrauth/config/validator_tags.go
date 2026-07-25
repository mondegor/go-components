package config

import (
	"github.com/mondegor/go-webcore/mrview"

	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/validate"
)

// TagEmail - возвращает тег валидации email.
func TagEmail() mrview.Tag {
	return mrview.Tag{
		Name:         "tag_email",
		ValidateFunc: contactaddress.ValidateEmail,
	}
}

// TagPhone - возвращает тег валидации телефона.
func TagPhone() mrview.Tag {
	return mrview.Tag{
		Name:         "tag_phone",
		ValidateFunc: contactaddress.ValidatePhone,
	}
}

// TagEmailPhone - возвращает тег валидации email или телефона.
func TagEmailPhone() mrview.Tag {
	return mrview.Tag{
		Name: "tag_email_phone",
		ValidateFunc: mrview.NewValidateOR(
			contactaddress.ValidateEmail,
			contactaddress.ValidatePhoneWorld,
		),
	}
}

// TagLang - возвращает тег валидации языка (локали) пользователя: проверяется форма записи
// и принадлежность языка списку langs, зарегистрированному приложением.
//
// Проверка строгая - ближайший поддерживаемый язык здесь не подбирается ("fr-CH" при
// поддержке "fr" совпадением не считается). Подбор уместен для клиентского Accept-Language,
// который о списке приложения не знает, и выполняется на разборе запроса; сюда же приходит
// значение, которое клиент выбрал явно и которое пойдёт в профиль, а затем - в область
// действия его токенов, поэтому оно обязано быть одним из langs.
func TagLang(langs []string) mrview.Tag {
	return mrview.Tag{
		Name: "tag_lang",
		ValidateFunc: mrview.NewValidateAND(
			validate.Lang,
			mrview.NewValidateInArray(langs),
		),
	}
}

// TagTimeZone - возвращает тег валидации IANA-имени часового пояса: проверяется формат имени
// и его принадлежность списку zones, зарегистрированному приложением. Имя тега - "tag_tz",
// по имени поля tz, которое им проверяется.
//
// Проверка строгая - подбор по смещению здесь не выполняется; он уместен для клиентского
// заголовка X-Accept-Time-Zone и выполняется на разборе запроса. Сюда же приходит имя,
// которое клиент выбрал явно и которое пойдёт в профиль, поэтому оно обязано быть одним
// из zones.
func TagTimeZone(zones []string) mrview.Tag {
	return mrview.Tag{
		Name: "tag_tz",
		ValidateFunc: mrview.NewValidateAND(
			validate.TimeZone,
			mrview.NewValidateInArray(zones),
		),
	}
}

// TagRealm - возвращает тег валидации realm из заданного списка.
func TagRealm(realms []string) mrview.Tag {
	return mrview.Tag{
		Name: "tag_realm",
		ValidateFunc: mrview.NewValidateAND(
			mrview.ValidateName,
			mrview.NewValidateInArray(realms),
		),
	}
}
