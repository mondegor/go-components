package mrordering

import (
	"github.com/mondegor/go-core/errors"
)

// ErrAfterNodeNotFound - after node with ID not found.
var ErrAfterNodeNotFound = errors.NewUserProto("AfterNodeNotFound", "after node with ID={Id} not found")
