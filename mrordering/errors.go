package mrordering

import "github.com/mondegor/go-sysmess/mrerr"

// ErrAfterNodeNotFound - after node with ID not found.
var ErrAfterNodeNotFound = mrerr.NewProto(
	"mrordering.errAfterNodeNotFound", mrerr.ErrorKindUser, "after node with ID={{ .id }} not found")
