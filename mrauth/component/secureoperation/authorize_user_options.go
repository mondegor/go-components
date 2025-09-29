package secureoperation

import (
	"github.com/mondegor/go-components/mrauth/component/secureoperation/action"
)

const (
	defaultConfirmPhoneByEmail = true
)

type (
	authorizeUserOptions struct {
		confirmByEmail      []action.Option
		confirmByPhone      []action.Option
		confirmPhoneByEmail bool
	}

	// AuthorizeUserOption - настройка объекта MessageSender.
	AuthorizeUserOption func(co *authorizeUserOptions)
)

// WithAuthorizeUserConfirmByEmailOpts - устанавливает кол-во попыток отправки одного сообщения.
func WithAuthorizeUserConfirmByEmailOpts(opts ...action.Option) AuthorizeUserOption {
	return func(co *authorizeUserOptions) {
		co.confirmByEmail = opts
	}
}

// WithAuthorizeUserConfirmByPhoneOpts - устанавливает кол-во попыток отправки одного сообщения.
func WithAuthorizeUserConfirmByPhoneOpts(opts ...action.Option) AuthorizeUserOption {
	return func(co *authorizeUserOptions) {
		co.confirmByPhone = opts
	}
}

// WithAuthorizeUserConfirmPhoneByEmail - устанавливает кол-во попыток отправки одного сообщения.
func WithAuthorizeUserConfirmPhoneByEmail(value bool) AuthorizeUserOption {
	return func(co *authorizeUserOptions) {
		co.confirmPhoneByEmail = value
	}
}
