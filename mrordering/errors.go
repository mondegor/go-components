package mrordering

import "github.com/mondegor/go-sysmess/mrerr"

// ErrAfterNodeNotFound - after node with SettingID not found.
var ErrAfterNodeNotFound = mrerr.NewKindUser("AfterNodeNotFound", "after node with SettingID={Id} not found")
