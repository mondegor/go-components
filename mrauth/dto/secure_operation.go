package dto

import "github.com/mondegor/go-core/mrtype"

type (
	// WaitingConfirmOperation struct {
	// 	Token             string
	// 	ConfirmMethod     confirmmethod.Enum
	// 	RemainingAttempts int16
	// 	RemainingResends  int16
	// 	ResendsAt         time.Time
	// 	ExpiresAt         time.Time
	// 	DebugInfo         string
	// }.

	// CreateUserOperation - payload операции создания пользователя.
	CreateUserOperation struct {
		Realm        string            `json:"realm"`
		UserKind     string            `json:"user_kind"`
		LangCode     string            `json:"lang_code"`
		TimeZone     string            `json:"timezone"`
		Email        string            `json:"email"`
		RegisteredIP mrtype.DetailedIP `json:"registered_ip"`
	}

	// AuthorizeUserOperation - payload операции авторизации пользователя.
	AuthorizeUserOperation struct {
		Realm    string `json:"realm"`
		LangCode string `json:"lang_code"`
	}

	// OperationWithUserEmail - общий payload операции, содержащий email для уведомления.
	OperationWithUserEmail struct {
		Email string `json:"email"`
	}

	// ChangeTOTPOperation - payload операции смены TOTP: email уведомления и
	// сгенерированный (ещё не привязанный) TOTP-secret.
	ChangeTOTPOperation struct {
		Email  string `json:"email"`
		Secret string `json:"secret,omitempty"`
	}

	// Disable2FAOperation - payload операции отключения 2FA.
	Disable2FAOperation OperationWithUserEmail

	// ChangeEmailOperation - payload операции смены email.
	ChangeEmailOperation struct {
		NewEmail string `json:"new_email"`
		Email    string `json:"email"`
	}

	// ChangePasswordOperation - payload операции смены пароля.
	ChangePasswordOperation struct {
		NewPassword string `json:"new_password"`
		Email       string `json:"email"`
	}

	// ChangePhoneOperation - payload операции смены телефона.
	ChangePhoneOperation struct {
		NewPhone uint64 `json:"new_phone"`
		Email    string `json:"email"`
	}
)
