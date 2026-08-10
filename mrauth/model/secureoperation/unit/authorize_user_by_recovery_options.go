package unit

import (
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

type (
	// AuthorizeUserByRecoveryOption - настройка объекта AuthorizeUserByRecovery.
	AuthorizeUserByRecoveryOption func(o *authorizeUserByRecoveryOptions)

	authorizeUserByRecoveryOptions struct {
		confirmByPassword []action.Option
		confirmByTOTP     []action.Option
		confirmByRecovery []action.Option
	}
)

// WithAuthorizeUserByRecoveryConfirmByPasswordOpts - настраивает действие подтверждения
// паролем; применяется к подставному действию цепочки для аккаунта без 2FA.
func WithAuthorizeUserByRecoveryConfirmByPasswordOpts(opts ...action.Option) AuthorizeUserByRecoveryOption {
	return func(o *authorizeUserByRecoveryOptions) {
		o.confirmByPassword = append(o.confirmByPassword, opts...)
	}
}

// WithAuthorizeUserByRecoveryConfirmByTOTPOpts - настраивает действие подтверждения
// TOTP-кодом; применяется к подставному действию цепочки для аккаунта без 2FA.
func WithAuthorizeUserByRecoveryConfirmByTOTPOpts(opts ...action.Option) AuthorizeUserByRecoveryOption {
	return func(o *authorizeUserByRecoveryOptions) {
		o.confirmByTOTP = append(o.confirmByTOTP, opts...)
	}
}

// WithAuthorizeUserByRecoveryConfirmByRecoveryOpts - настраивает завершающее действие
// подтверждения аварийным кодом.
func WithAuthorizeUserByRecoveryConfirmByRecoveryOpts(opts ...action.Option) AuthorizeUserByRecoveryOption {
	return func(o *authorizeUserByRecoveryOptions) {
		o.confirmByRecovery = append(o.confirmByRecovery, opts...)
	}
}
