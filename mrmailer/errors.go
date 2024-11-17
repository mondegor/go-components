package mrmailer

import (
	"github.com/mondegor/go-sysmess/mrerr"
)

var (
	// ErrCheckMessageHasNotData - message data is not specified.
	ErrCheckMessageHasNotData = mrerr.NewProto(
		"mrmailer.errCheckMessageHasNotData", mrerr.ErrorKindInternal, "data is not specified for message {{ .name }}")

	// ErrCheckMessageHasAFewData - only one message data is expected.
	ErrCheckMessageHasAFewData = mrerr.NewProto(
		"mrmailer.errCheckMessageHasAFewData", mrerr.ErrorKindInternal, "only one data is expected for message {{ .name }}")

	// ErrProviderClientNotSpecified - there is no provider client to send this message of type.
	ErrProviderClientNotSpecified = mrerr.NewProto(
		"mrmailer.errProviderClientNotSpecified", mrerr.ErrorKindInternal, "there is no provider client to send this message of type {{ .type }}")
)
