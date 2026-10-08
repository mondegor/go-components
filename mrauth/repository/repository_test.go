package repository_test

// Идентификаторы пользователей, заданные в фикстурах testdata: тест ссылается на них,
// чтобы найти строки, подготовленные фикстурой.
const (
	fixtureUserA = "a0000000-0000-4000-8000-00000000000a"
	fixtureUserB = "b0000000-0000-4000-8000-00000000000b"
	fixtureUserC = "c0000000-0000-4000-8000-00000000000c" // не встречается ни в одной фикстуре
)

// Идентификаторы realm'ов в фикстурах и тестах.
const (
	realmA uint16 = 1
	realmB uint16 = 2
)

// Таблицы, к которым обращаются тесты нескольких репозиториев.
const (
	sessionsTableName   = "sample_schema.sessions"
	authTokensTableName = "sample_schema.auth_tokens"
)
