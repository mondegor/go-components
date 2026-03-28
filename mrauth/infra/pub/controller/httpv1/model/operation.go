package model

import (
	"github.com/mondegor/go-webcore/mrserver/mrresp"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// OperationTokenRequest - запрос на подтверждение операции.
	OperationTokenRequest struct {
		Token string `json:"token" validate:"required,min=64,max=128"`
	}

	// ConfirmOperationRequest - запрос на подтверждение операции.
	ConfirmOperationRequest struct {
		Token  string `json:"token" validate:"required,min=64,max=128"`
		Secret string `json:"secret" validate:"required,min=4,max=32"`
	}

	// WaitingConfirmOperationResponse - информация для подтверждения операции.
	WaitingConfirmOperationResponse struct {
		Token             string             `json:"token"`
		ConfirmMethod     confirmmethod.Enum `json:"confirm_method"`
		RemainingAttempts int16              `json:"remaining_attempts"`
		RemainingResends  int16              `json:"remaining_resends,omitempty"`
		ResendsIn         int64              `json:"resends_in,omitempty"`
		ExpiresIn         int64              `json:"expires_in"`
		Message           string             `json:"message,omitempty"`
		DebugInfo         string             `json:"debug_info,omitempty"`
	}

	// ErrorConfirmOperationResponse - .
	ErrorConfirmOperationResponse struct {
		OperationStatus ConfirmOperationStatus  `json:"operation_status,omitempty"`
		Errors          []mrresp.ErrorAttribute `json:"errors"`
	}

	// ConfirmOperationStatus - информация об оставшихся попытках и времени действия операции.
	// Поля RemainingResends и ResendsIn не используются для пароля и TOTP.
	ConfirmOperationStatus struct {
		RemainingAttempts int16  `json:"remaining_attempts"`
		RemainingResends  int16  `json:"remaining_resends"`
		ResendsIn         int64  `json:"resends_in"`
		ExpiresIn         int64  `json:"expires_in"`
		DebugInfo         string `json:"debug_info,omitempty"`
	}
)
