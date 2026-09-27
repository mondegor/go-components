package model

import (
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
)

type (
	// CreateUserRequest - запрос на создание нового пользователя.
	CreateUserRequest struct {
		Realm     string `json:"realm" validate:"required,min=4,max=32,tag_realm"`
		UserEmail string `json:"user_email" validate:"required,min=7,max=64,tag_email"`
	}

	// AuthorizeUserRequest - запрос на авторизацию пользователя в системе.
	AuthorizeUserRequest struct {
		Realm     string `json:"realm" validate:"required,min=4,max=32,tag_realm"`
		UserLogin string `json:"user_login" validate:"required,min=7,max=64,tag_email_phone"`
	}

	// LoginByTokenRequest - запрос на авторизацию пользователя в системе.
	// Secret не передаётся, когда подтверждение секретом не требуется.
	LoginByTokenRequest struct {
		Token  string  `json:"token" validate:"required,min=64,max=128"`
		Secret *string `json:"secret,omitempty" validate:"omitnil,min=4,max=32"`
	}

	// ContinueSessionRequest - запрос на продление текущей сессии по refresh токену.
	ContinueSessionRequest struct {
		RefreshToken string `json:"refresh_token" validate:"required,min=64,max=128"`
	}

	// CloseSessionRequest - запрос на закрытие сессии (logout) по refresh токену.
	CloseSessionRequest struct {
		RefreshToken string `json:"refresh_token" validate:"required,min=64,max=128"`
	}

	// ChangeSettingsRequest - запрос на изменение языка и часового пояса пользователя.
	// Отсутствующее поле - режим "авто" (см. Auth.ChangeSettings).
	ChangeSettingsRequest struct {
		LangCode *string `json:"lang,omitempty" validate:"omitnil,min=2,max=5,tag_lang"`
		TimeZone *string `json:"tz,omitempty" validate:"omitnil,min=3,max=64,tag_tz"`
	}

	// ChangeSettingsResponse - настройки пользователя, которые реально сохранены.
	// Для поля, отсутствовавшего в запросе, здесь возвращается подобранное значение,
	// поэтому клиент применяет у себя именно эти.
	ChangeSettingsResponse struct {
		LangCode string `json:"lang"`
		TimeZone string `json:"tz"`
	}

	// SuccessAccessResponse - ответ с выданной парой токенов доступа к аккаунту.
	SuccessAccessResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    uint32 `json:"expires_in"`
		RefreshToken string `json:"refresh_token,omitempty"` // can be in cookie
	}

	// UserInfoResponse - ответ со сводной информацией о текущем пользователе.
	// RecoveryCodesLeft - кол-во оставшихся аварийных кодов отдаётся только при включённой 2FA.
	UserInfoResponse struct {
		Email             string             `json:"email"`
		Phone             string             `json:"phone,omitempty"`
		LangCode          string             `json:"lang"`
		TimeZone          string             `json:"tz"`
		Auth2FAType       auth2fatype.Enum   `json:"auth_2fa_type"`
		RecoveryCodesLeft *int               `json:"recovery_codes_left,omitempty"`
		Realms            []UserRealm        `json:"realms"`
		PendingOperations []PendingOperation `json:"pending_operations,omitempty"`
		Status            userstatus.Enum    `json:"status"`
	}

	// PendingOperation - действующая операция личного кабинета, ожидающая подтверждения либо
	// применения. ExtraValue - новый емаил или телефон у операций их смены.
	PendingOperation struct {
		Token             string               `json:"token"`
		Type              operationtype.Enum   `json:"type"`
		ExtraValue        string               `json:"extra_value,omitempty"`
		ExpiresAt         string               `json:"expires_at"`
		Status            operationstatus.Enum `json:"status"`
		ConfirmMethod     confirmmethod.Enum   `json:"confirm_method,omitempty"`
		RemainingAttempts *int16               `json:"remaining_attempts,omitempty"`
		RemainingResends  *int16               `json:"remaining_resends,omitempty"`
		ResendsIn         *int64               `json:"resends_in,omitempty"`
	}

	// UserRealm - realm пользователя с его видом и статистикой последнего входа
	// в ответе с информацией о пользователе.
	UserRealm struct {
		Name         string `json:"name"`
		UserKind     string `json:"user_kind"`
		LastLocation string `json:"last_location,omitempty"`
		LastLoggedAt string `json:"last_logged_at,omitempty"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}
)
