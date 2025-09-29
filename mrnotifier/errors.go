package mrnotifier

import "github.com/mondegor/go-sysmess/mrerr"

// ErrTemplateNotRegistered - no template is registered for the notification with lang.
var ErrTemplateNotRegistered = mrerr.NewKindInternal("no template is registered for the notification with lang (template={Realm}, lang={Lang}")
