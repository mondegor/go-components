package mrmailer

import "github.com/mondegor/go-sysmess/mrerr"

var (
	// ErrCheckMessageHasNotData - message data is not specified.
	ErrCheckMessageHasNotData = mrerr.NewKindInternal("data is not specified for message: '{Realm}'")

	// ErrCheckMessageHasAFewData - only one message data is expected.
	ErrCheckMessageHasAFewData = mrerr.NewKindInternal("only one data is expected for message: '{Realm}'")

	// ErrProviderClientNotSpecified - there is no provider client to send this message of type.
	ErrProviderClientNotSpecified = mrerr.NewKindInternal("there is no provider client to send this message of type: '{Type}'")
)
