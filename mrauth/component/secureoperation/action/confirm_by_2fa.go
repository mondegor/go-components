package action

import (
	"errors"

	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
)

type (
	// ConfirmBy2fa - comment struct.
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

// Create - comments method.
func (a *ConfirmBy2fa) Create(auth2fa auth2fatype.Enum, secret string) (dto.ConfirmAction, error) {
	if auth2fa == auth2fatype.Password {
		return a.confirmByPassword.Create(secret), nil
	}

	if auth2fa == auth2fatype.TOTP {
		return a.confirmByTOTP.Create(secret), nil
	}

	return dto.ConfirmAction{}, mr.ErrInternal.Wrap(errors.New("invalid auth2fa type")).WithAttr("auth2fa", auth2fa)
}
