package model

import (
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
)

type (
	// CreateUserRequest - запрос на создание нового пользователя.
	CreateUserRequest struct {
		Realm     string `json:"realm" validate:"required,max=32,tag_realm"`
		UserEmail string `json:"user_email" validate:"required,min=7,max=64,tag_email"`
	}

	// AuthorizeUserRequest - запрос на авторизацию пользователя в системе.
	AuthorizeUserRequest struct {
		Realm     string `json:"realm" validate:"required,max=32,tag_realm"`
		UserLogin string `json:"user_login" validate:"required,min=7,max=64,tag_email_phone"`
	}

	// LoginByTokenRequest - запрос на авторизацию пользователя в системе.
	LoginByTokenRequest struct {
		Token  string `json:"token" validate:"required,min=64,max=128"`
		Secret string `json:"secret,omitempty" validate:"omitempty,min=4,max=32"`
	}

	// ContinueSessionRequest - запрос на продление текущей сессии по refresh токену.
	ContinueSessionRequest struct {
		RefreshToken string `json:"refresh_token" validate:"required,min=64,max=128"`
	}

	// CloseSessionRequest - запрос на закрытие сессии (logout) по refresh токену.
	CloseSessionRequest struct {
		RefreshToken string `json:"refresh_token" validate:"required,min=64,max=128"`
	}

	// SuccessAccessResponse - ответ с выданной парой токенов доступа к аккаунту.
	SuccessAccessResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    uint32 `json:"expires_in"`
		RefreshToken string `json:"refresh_token,omitempty"` // can be in cookie
		Message      string `json:"message,omitempty"`       // OPTIONAL
	}

	// UserInfoResponse - ответ со сводной информацией о текущем пользователе.
	UserInfoResponse struct {
		Email       string           `json:"email"`
		Phone       string           `json:"phone,omitempty"`
		LangCode    string           `json:"lang"`
		Auth2FAType auth2fatype.Enum `json:"auth_2fa_type"`
		Realms      []UserRealm      `json:"realms"`
		Status      userstatus.Enum  `json:"status"`
	}

	// UserRealm - realm пользователя с его видом и статистикой последнего входа
	// в ответе с информацией о пользователе. LastLoggedAt отсутствует, если пользователь
	// ни разу не входил в этот realm; LastLocation - в этом же случае, а также
	// когда местоположение не определено (IP входа не сохранён или не распознан).
	UserRealm struct {
		Name         string `json:"name"`
		UserKind     string `json:"user_kind"`
		LastLocation string `json:"last_location,omitempty"`
		LastLoggedAt string `json:"last_logged_at,omitempty"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}
)
