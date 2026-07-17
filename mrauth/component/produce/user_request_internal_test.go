package produce

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// allowUnknownRealmLog проверяется internal-тестом: троттлинг завязан на времени, а промотать
// его через публичный API нельзя - now передаётся в метод параметром.
// Тест последовательный, а не табличный: каждый шаг опирается на состояние, оставленное
// предыдущим, поэтому шаги нельзя ни переставить, ни выполнить параллельно.
func TestUserRequest_allowUnknownRealmLog(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	rs := &UserRequest{}

	assert.True(t, rs.allowUnknownRealmLog(now), "первый промах реестра сообщается сразу")

	assert.False(
		t,
		rs.allowUnknownRealmLog(now.Add(unknownRealmLogPeriod-time.Second)),
		"повтор в пределах периода молчит",
	)

	assert.True(
		t,
		rs.allowUnknownRealmLog(now.Add(unknownRealmLogPeriod)),
		"по истечении периода сообщается снова",
	)

	assert.False(
		t,
		rs.allowUnknownRealmLog(now.Add(unknownRealmLogPeriod+time.Second)),
		"после протухания период отсчитывается заново",
	)

	assert.True(
		t,
		rs.allowUnknownRealmLog(now.Add(2*unknownRealmLogPeriod)),
		"истёк и второй период",
	)
}
