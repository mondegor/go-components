package action

import (
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
)

type (
	// ConfirmBy2fa - фабрика действий подтверждения вторым фактором (пароль или TOTP).
	ConfirmBy2fa struct {
		confirmByPassword *ConfirmByPassword
		confirmByTOTP     *ConfirmByTOTP
	}
)

// NewConfirmBy2fa - создаёт объект ConfirmBy2fa.
func NewConfirmBy2fa(passwordOpts, totpOpts []Option) *ConfirmBy2fa {
	return &ConfirmBy2fa{
		confirmByPassword: NewConfirmByPassword(passwordOpts...),
		confirmByTOTP:     NewConfirmByTOTP(totpOpts...),
	}
}

// Create - создаёт описание второго фактора по указанному типу 2FA.
func (a *ConfirmBy2fa) Create(auth2fa auth2fatype.Enum) (dto.ConfirmAction2FA, error) {
	if auth2fa == auth2fatype.Password {
		return a.confirmByPassword.Create(), nil
	}

	if auth2fa == auth2fatype.TOTP {
		return a.confirmByTOTP.Create(), nil
	}

	return dto.ConfirmAction2FA{},
		errors.NewInternalError(
			"auth2fa type is invalid",
			"auth2fa", auth2fa,
		)
}
