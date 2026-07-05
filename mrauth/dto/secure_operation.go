package dto

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
		Realm        string `json:"realm"`
		UserKind     string `json:"user_kind"`
		LangCode     string `json:"lang_code"`
		Email        string `json:"email"`
		RegisteredIP string `json:"registered_ip"`
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

	// ChangeTotpOperation - payload операции смены TOTP: email уведомления и
	// сгенерированный (ещё не привязанный) TOTP-secret.
	ChangeTotpOperation struct {
		Email  string `json:"email"`
		Secret string `json:"secret,omitempty"`
	}

	// Disable2faOperation - payload операции отключения 2FA.
	Disable2faOperation OperationWithUserEmail

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
