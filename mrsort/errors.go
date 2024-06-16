package mrsort

import e "github.com/mondegor/go-sysmess/mrerr"

// ErrAfterNodeNotFound - after node with ID not found.
var ErrAfterNodeNotFound = e.NewProto(
	"errMrSortAfterNodeNotFound", e.ErrorKindUser, "after node with ID={{ .id }} not found")
