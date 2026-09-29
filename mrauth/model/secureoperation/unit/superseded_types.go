package unit

import (
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
)

// SupersededTypes - возвращает типы операций, которые гасит открытие операции типа opType.
// Операции одной цепочки (шаги смены емаила) вытесняют друг друга: иначе у пользователя
// одновременно живут две ветки цепочки, и начатая до смены адреса доводится после неё.
func SupersededTypes(opType operationtype.Enum) []operationtype.Enum {
	switch opType {
	case operationtype.ChangeEmail, operationtype.ChangeEmailConfirm:
		return []operationtype.Enum{operationtype.ChangeEmail, operationtype.ChangeEmailConfirm}
	default:
		return []operationtype.Enum{opType}
	}
}
