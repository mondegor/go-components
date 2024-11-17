package mrnotifier

import (
	"github.com/mondegor/go-sysmess/mrerr"
)

// ErrTemplateNotRegistered - no template is registered for the notification with lang.
var ErrTemplateNotRegistered = mrerr.NewProto(
	"mrnotifier.errTemplateNotRegistered", mrerr.ErrorKindInternal, "no template is registered for the notification {{ .name }} with lang {{ .lang }}")
