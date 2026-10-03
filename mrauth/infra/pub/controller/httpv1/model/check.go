package model

import (
	"github.com/mondegor/go-components/mrauth/enum/passwordacceptstatus"
)

type (
	// CheckLoginRequest - запрос на проверку свободен ли указанный емаил/телефон.
	CheckLoginRequest struct {
		Realm     string `json:"realm" validate:"required,min=4,max=32,tag_realm"`
		UserLogin string `json:"user_login" validate:"required,min=7,max=64,tag_email_phone"`
	}

	// CalcPasswordStrengthRequest - запрос на проверку надёжности указанного пароля.
	CalcPasswordStrengthRequest struct {
		Password string `json:"password" validate:"required,min=10,max=32,tag_password"`
	}

	// CalcPasswordStrengthResponse - информация о надёжности пароля.
	// AcceptStatus - примет ли пароль установка пароля 2FA, а если нет - причина отказа.
	CalcPasswordStrengthResponse struct {
		Strength     string                    `json:"strength"`
		AcceptStatus passwordacceptstatus.Enum `json:"accept_status"`
	}

	// GeneratedPasswordResponse - сгенерированный пароль.
	GeneratedPasswordResponse struct {
		Password string `json:"password"`
	}
)
