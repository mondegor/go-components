package mrorderer

import . "github.com/mondegor/go-sysmess/mrerr"

var (
	FactoryErrAfterNodeNotFound = NewFactory(
		"errMrOrderAfterNodeNotFound", ErrorKindUser, "after node with ID={{ .id }} not found")
)
