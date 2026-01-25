package secureoperation

import (
	"github.com/mondegor/go-components/mrauth/component/secureoperation/action"
)

const (
	defaultConfirmPhoneByEmail = true
)

type (
	// AuthorizeUserOption - настройка объекта MessageSender.
	AuthorizeUserOption func(o *authorizeUserOptions)

	authorizeUserOptions struct {
		authorizer     *AuthorizeUser
		confirmByEmail []action.Option
		confirmByPhone []action.Option
	}
)

// WithAuthorizeUserConfirmByEmailOpts - устанавливает кол-во попыток отправки одного сообщения.
func WithAuthorizeUserConfirmByEmailOpts(opts ...action.Option) AuthorizeUserOption {
	return func(o *authorizeUserOptions) {
		o.confirmByEmail = append(o.confirmByEmail, opts...)
	}
}

// WithAuthorizeUserConfirmByPhoneOpts - устанавливает кол-во попыток отправки одного сообщения.
func WithAuthorizeUserConfirmByPhoneOpts(opts ...action.Option) AuthorizeUserOption {
	return func(o *authorizeUserOptions) {
		o.confirmByPhone = append(o.confirmByPhone, opts...)
	}
}

// WithAuthorizeUserConfirmPhoneByEmail - устанавливает кол-во попыток отправки одного сообщения.
func WithAuthorizeUserConfirmPhoneByEmail(value bool) AuthorizeUserOption {
	return func(o *authorizeUserOptions) {
		o.authorizer.confirmPhoneByEmail = value
	}
}
