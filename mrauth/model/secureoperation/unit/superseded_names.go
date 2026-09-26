package unit

// SupersededNames - возвращает имена операций, которые гасит открытие операции name.
// Операции одной цепочки (шаги смены емаила) вытесняют друг друга: иначе у пользователя
// одновременно живут две ветки цепочки, и начатая до смены адреса доводится после неё.
func SupersededNames(name string) []string {
	switch name {
	case NameConfirmChangeEmailRequest, NameConfirmChangeEmail:
		return []string{NameConfirmChangeEmailRequest, NameConfirmChangeEmail}
	default:
		return []string{name}
	}
}
