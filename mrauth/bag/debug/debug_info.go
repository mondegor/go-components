package debug

import (
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// Info - comment func.
func Info(op secureoperation.SecureOperation) string {
	action, ok := op.FirstAction()
	if !ok {
		return ""
	}

	info := "Method: " + action.Method.String()

	if action.Sendable() {
		info += ", to: " + action.Address + ", code: " + action.PlainConfirmCode
	}

	return info
}
