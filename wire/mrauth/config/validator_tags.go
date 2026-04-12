package config

import (
	"github.com/mondegor/go-webcore/mrview"

	"github.com/mondegor/go-components/mrauth/model/contactaddress"
)

// TagEmail - comment func.
func TagEmail() mrview.Tag {
	return mrview.Tag{
		Name:         "tag_email",
		ValidateFunc: contactaddress.ValidateEmail,
	}
}

// TagPhone - comment func.
func TagPhone() mrview.Tag {
	return mrview.Tag{
		Name:         "tag_phone",
		ValidateFunc: contactaddress.ValidatePhone,
	}
}

// TagEmailPhone - comment func.
func TagEmailPhone() mrview.Tag {
	return mrview.Tag{
		Name: "tag_email_phone",
		ValidateFunc: mrview.NewValidateOR(
			contactaddress.ValidateEmail,
			contactaddress.ValidatePhoneWorld,
		),
	}
}

// TagRealm - comment func.
func TagRealm(realms []string) mrview.Tag {
	return mrview.Tag{
		Name: "tag_realm",
		ValidateFunc: mrview.NewValidateAND(
			mrview.ValidateName,
			mrview.NewValidateInArray(realms),
		),
	}
}
