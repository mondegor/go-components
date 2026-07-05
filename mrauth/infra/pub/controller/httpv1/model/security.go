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

	// ApplyOperationRequest - запрос на подтверждение операции.
	ApplyOperationRequest struct {
		Token string `json:"token" validate:"required,min=64,max=128"`
	}

	// ChangePasswordRequest - запрос на установку/изменение пароля пользователя (2FA).
	ChangePasswordRequest struct {
		NewPassword string `json:"new_password" validate:"required,min=8,max=32,tag_password"`
	}

	// ApplyPasswordRequest - запрос на применение подтверждённой операции смены пароля
	// (привязка пароля как 2FA и выдача аварийных кодов).
	ApplyPasswordRequest struct {
		Token string `json:"token" validate:"required,min=64,max=128"`
	}

	// ApplyTOTPGeneratorRequest - запрос на проверку TOTP-кода и привязку генератора.
	// Метод принимает только 6-значный цифровой TOTP-код (аварийные коды здесь не используются).
	ApplyTOTPGeneratorRequest struct {
		Token string `json:"token" validate:"required,min=64,max=128"`
		Code  string `json:"totp_code" validate:"required,len=6,numeric"`
	}

	// ApplyRecoveryCodesRequest - запрос на применение подтверждённой операции перевыпуска
	// аварийных кодов (выдача нового набора кодов).
	ApplyRecoveryCodesRequest struct {
		Token string `json:"token" validate:"required,min=64,max=128"`
	}

	// RecoveryCodesResponse - выданные одноразовые аварийные коды (показываются один раз).
	RecoveryCodesResponse struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
)
