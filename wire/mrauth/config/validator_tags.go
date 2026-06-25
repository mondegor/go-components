package config

import (
	"github.com/mondegor/go-webcore/mrview"

	"github.com/mondegor/go-components/mrauth/model/contactaddress"
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
