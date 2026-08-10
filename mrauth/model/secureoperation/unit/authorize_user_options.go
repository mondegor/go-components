package unit

import (
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

const (
	defaultConfirmPhoneByEmail = true
)

type (
	// AuthorizeUserOption - настройка объекта AuthorizeUser.
	AuthorizeUserOption func(o *authorizeUserOptions)

	authorizeUserOptions struct {
		confirmByEmail      []action.Option
		confirmByPhone      []action.Option
		confirmPhoneByEmail bool
	}
)

// WithAuthorizeUserConfirmByEmailOpts - настраивает действие подтверждения кодом с емаила.
func WithAuthorizeUserConfirmByEmailOpts(opts ...action.Option) AuthorizeUserOption {
	return func(o *authorizeUserOptions) {
		o.confirmByEmail = append(o.confirmByEmail, opts...)
	}
}

// WithAuthorizeUserConfirmByPhoneOpts - настраивает действие подтверждения кодом с телефона.
func WithAuthorizeUserConfirmByPhoneOpts(opts ...action.Option) AuthorizeUserOption {
	return func(o *authorizeUserOptions) {
		o.confirmByPhone = append(o.confirmByPhone, opts...)
	}
}

// WithAuthorizeUserConfirmPhoneByEmail - включает отправку кода подтверждения на емаил
// пользователя, даже когда он входит по номеру телефона.
func WithAuthorizeUserConfirmPhoneByEmail(value bool) AuthorizeUserOption {
	return func(o *authorizeUserOptions) {
		o.confirmPhoneByEmail = value
	}
}
