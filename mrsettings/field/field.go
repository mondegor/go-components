package field

type (
	// ValueParser - парсер значения настройки полученного из хранилища,
	// с целью приведения к нужному типу данных.
	ValueParser interface {
		ParseString(value string) (string, error)
		ParseStringList(value string) ([]string, error)
		ParseInt64(value string) (int64, error)
		ParseInt64List(value string) ([]int64, error)
		ParseBool(value string) (bool, error)
	}

	// ValueFormatter - форматер значения настройки, который подготавливает
	// его к сохранению в хранилище данных. Если необходима валидация данных,
	// то она должна происходить до этапа форматирования.
	ValueFormatter interface {
		FormatString(value string) (string, error)
		FormatStringList(values []string) (string, error)
		FormatInt64(value int64) (string, error)
		FormatInt64List(values []int64) (string, error)
		FormatBool(value bool) (string, error)
	}
)
