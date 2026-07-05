package mrmailer

import (
	"github.com/mondegor/go-core/errors"
)

var (
	// ErrInternalCheckMessageHasNotData - message data is not specified (attr: channel).
	ErrInternalCheckMessageHasNotData = errors.NewInternalProto("data is not specified for message")

	// ErrInternalCheckMessageHasAFewData - only one message data is expected (attr: channel).
	ErrInternalCheckMessageHasAFewData = errors.NewInternalProto("only one data is expected for message")

	// ErrInternalProviderClientNotSpecified - there is no provider client to send this message of type (attrs: channel, type).
	ErrInternalProviderClientNotSpecified = errors.NewInternalProto("there is no provider client to send this message of type")
)
