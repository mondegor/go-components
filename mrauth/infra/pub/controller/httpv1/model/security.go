package model

import (
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
)

type (
	// ChangeEmailRequest - запрос на изменение емаила пользователя.
	ChangeEmailRequest struct {
		NewEmail string `json:"new_email" validate:"required,min=7,max=64,tag_email"`
	}

	// ApplyEmailRequest - запрос на применение подтверждённой операции первого шага смены
	// емаила (открытие операции подтверждения владения новым адресом).
	ApplyEmailRequest struct {
		Token string `json:"token" validate:"required,min=64,max=128"`
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
		NewPassword string `json:"new_password" validate:"required,min=10,max=32,tag_password"`
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
		Code  string `json:"totp_code" validate:"required,len=6,number"`
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

	// TOTPGeneratorSecretResponse - заготовка TOTP-генератора в текстовом виде: secret для
	// ручного ввода и otpauth-ссылка с теми же параметрами, что закодированы в QR-коде.
	TOTPGeneratorSecretResponse struct {
		Secret     string `json:"secret"`
		OTPAuthURI string `json:"otpauth_uri"`
	}

	// SecurityLogResponse - страница журнала безопасности пользователя, от свежих событий к старым.
	SecurityLogResponse struct {
		Items   []SecurityLogItem `json:"items"`
		Cursor  string            `json:"cursor"`
		HasNext bool              `json:"has_next"`
	}

	// SecurityLogItem - событие журнала безопасности пользователя.
	SecurityLogItem struct {
		ID         string             `json:"id"`
		EventType  securityevent.Enum `json:"event_type"`
		IP         string             `json:"ip"`
		Location   string             `json:"location,omitempty"`
		AppName    string             `json:"app_name"`
		DeviceName string             `json:"device_name"`
		OldValue   string             `json:"old_value,omitempty"`
		NewValue   string             `json:"new_value,omitempty"`
		Factor     string             `json:"factor,omitempty"`
		Remaining  *int               `json:"remaining,omitempty"`
		CreatedAt  string             `json:"created_at"`
	}
)
