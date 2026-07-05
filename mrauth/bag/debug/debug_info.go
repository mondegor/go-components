package debug

import (
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// Info - формирует отладочную строку по текущему действию операции.
// Открытый код подтверждения (PlainConfirmCode) добавляется только при showSecret = true.
func Info(op secureoperation.SecureOperation, showSecret bool) string {
	action, ok := op.FirstAction()
	if !ok {
		return ""
	}

	info := "Method: " + action.Method.String()

	if action.Sendable() {
		info += ", to: " + action.Address

		if showSecret {
			info += ", code: " + action.PlainConfirmCode
		}
	}

	return info
}
