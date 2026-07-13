package security

import (
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
)

type (
	// logState - намеченный исход события для журнала. Статус и причина хранятся вместе,
	// поэтому ветка не может выставить одно без другого и записать в журнал невалидный
	// нулевой статус (logstatus нумеруется с 1).
	logState struct {
		status logstatus.Enum
		reason logreason.Enum
	}
)

func newLogState(status logstatus.Enum, reason logreason.Enum) logState {
	return logState{
		status: status,
		reason: reason,
	}
}

// isSet - сообщает, был ли исход намечен (т.е. требуется ли запись в журнал).
func (o logState) isSet() bool {
	return o.status != 0
}
