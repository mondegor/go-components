package mrnotifier

import (
	"github.com/mondegor/go-core/errors"
)

// ErrSystemTemplateNotRegistered - no template is registered for the notification with lang (attrs: template, lang, status).
var ErrSystemTemplateNotRegistered = errors.NewSystemProto("no template is registered for the notification with lang")
