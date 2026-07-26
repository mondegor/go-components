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
	// RemainingResends и ResendsIn отдаются вместе и только когда повторные отправки
	// применимы (емаил и телефон); для пароля и TOTP их в ответе нет. Ноль в них -
	// значение, а не отсутствие: RemainingResends=0 - отправки исчерпаны,
	// ResendsIn=0 - повторную отправку можно сделать прямо сейчас.
	WaitingConfirmOperationResponse struct {
		Token             string             `json:"token"`
		ConfirmMethod     confirmmethod.Enum `json:"confirm_method"`
		RemainingAttempts int16              `json:"remaining_attempts"`
		RemainingResends  *int16             `json:"remaining_resends,omitempty"`
		ResendsIn         *int64             `json:"resends_in,omitempty"`
		ExpiresIn         int64              `json:"expires_in"`
		Message           string             `json:"message,omitempty"`
		DebugInfo         string             `json:"debug_info,omitempty"`
	}

	// ErrorConfirmOperationResponse - ответ с ошибкой подтверждения операции и её текущим состоянием.
	ErrorConfirmOperationResponse struct {
		mrresp.Error400Response

		OperationState ConfirmOperationState `json:"operation_state"`
	}

	// ConfirmOperationState - информация об оставшихся попытках и времени действия операции.
	// RemainingResends и ResendsIn отдаются вместе и только когда повторные отправки
	// применимы (емаил и телефон); для пароля и TOTP их в ответе нет. Ноль в них -
	// значение, а не отсутствие: RemainingResends=0 - отправки исчерпаны,
	// ResendsIn=0 - повторную отправку можно сделать прямо сейчас.
	ConfirmOperationState struct {
		RemainingAttempts int16  `json:"remaining_attempts"`
		RemainingResends  *int16 `json:"remaining_resends,omitempty"`
		ResendsIn         *int64 `json:"resends_in,omitempty"`
		ExpiresIn         int64  `json:"expires_in"`
		DebugInfo         string `json:"debug_info,omitempty"`
	}
)
