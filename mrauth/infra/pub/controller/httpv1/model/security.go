package model

type (
	// ChangeEmailRequest - запрос на изменение емаила пользователя.
	ChangeEmailRequest struct {
		NewEmail string `json:"new_email" validate:"required,min=7,max=64,tag_email"`
	}

	// ChangePhoneRequest - запрос на установку/изменение телефона пользователя.
	ChangePhoneRequest struct {
		NewPhone string `json:"new_phone" validate:"required,min=10,max=32,tag_phone"`
	}

	// ChangePasswordRequest - запрос на установку/изменение пароля пользователя (2FA).
	ChangePasswordRequest struct {
		NewPassword string `json:"new_password" validate:"required,min=8,max=32,tag_password"`
	}

	// ApplyOperationRequest - запрос на подтверждение операции.
	ApplyOperationRequest struct {
		Token string `json:"token" validate:"required,min=64,max=128"`
	}

	// ApplyTOTPGeneratorRequest - запрос на проверку TOTP-кода и привязку генератора.
	ApplyTOTPGeneratorRequest struct {
		Token string `json:"token" validate:"required,min=64,max=128"`
		Code  string `json:"totp_code" validate:"required,min=6,max=10"`
	}

	// GeneratedPasswordResponse - информация о надёжности пароля.
	GeneratedPasswordResponse struct {
		Password string `json:"password"`
	}

	// RecoveryCodesResponse - выданные одноразовые аварийные коды (показываются один раз).
	RecoveryCodesResponse struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
)
