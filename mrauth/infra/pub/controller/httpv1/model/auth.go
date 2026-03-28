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

	// ContinueSessionRequest - запрос на подтверждение операции.
	ContinueSessionRequest struct {
		RefreshToken string `json:"refresh_token" validate:"required,min=64,max=128"`
	}

	// SuccessAccessResponse - запрос на авторизацию пользователя в системе.
	SuccessAccessResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    uint32 `json:"expires_in"`
		RefreshToken string `json:"refresh_token,omitempty"` // can be in cookie
		Message      string `json:"message,omitempty"`       // OPTIONAL
	}

	// UserInfoResponse - запрос на авторизацию пользователя в системе.
	UserInfoResponse struct {
		Email        string           `json:"email"`
		Phone        string           `json:"phone,omitempty"`
		LangCode     string           `json:"lang"`
		LastLoginIP  string           `json:"last_login_ip"`
		LastLoggedAt string           `json:"last_logged_at"`
		Auth2faType  auth2fatype.Enum `json:"auth_2fa_type"`
		Realms       []UserRealm      `json:"realms"`
		Status       userstatus.Enum  `json:"status"`
		// CreatedAt    time.Time       `json:"created_at"`
		// UpdatedAt    time.Time       `json:"updated_at"`
	}

	// UserRealm - запрос на авторизацию пользователя в системе.
	UserRealm struct {
		Name     string `json:"name"`
		UserKind string `json:"user_kind"`
		// CreatedAt time.Time `json:"created_at"`
		// UpdatedAt time.Time `json:"updated_at"`
	}
)
