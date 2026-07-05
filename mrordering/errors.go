package mrordering

import (
	"github.com/mondegor/go-sysmess/errors"
)

// ErrAfterNodeNotFound - after node with ID not found.
var ErrAfterNodeNotFound = errors.NewUserProto("AfterNodeNotFound", "after node with ID={Id} not found")
