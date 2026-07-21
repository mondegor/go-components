package timezone_test

import (
	"testing"
	"time"
	_ "time/tzdata" // база зон встраивается в тест, чтобы он не зависел от её наличия в системе

	"github.com/mondegor/go-core/util/timezone"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/dto"
	servicetimezone "github.com/mondegor/go-components/mrauth/service/timezone"
)

// newLocations - создаёт список поясов, используемый во всех тестах пакета.
func newLocations(t *testing.T) *timezone.LocationList {
	t.Helper()

	return timezone.NewLocationList([]string{"Europe/Moscow", "Europe/Berlin", "Asia/Tokyo"})
}

// offsetOf - возвращает текущее смещение указанного пояса и признак летнего времени.
// Значения берутся у самого пояса, чтобы тест не зависел от времени года.
func offsetOf(t *testing.T, name string) (time.Duration, bool) {
	t.Helper()

	loc, err := time.LoadLocation(name)
	require.NoError(t, err)

	now := time.Now().In(loc)
	_, offset := now.Zone()

	return time.Duration(offset) * time.Second, now.IsDST()
}

func TestResolver_Resolve(t *testing.T) {
	t.Parallel()

	locations := newLocations(t)
	tokyoOffset, tokyoIsDST := offsetOf(t, "Asia/Tokyo")

	tests := []struct {
		name string
		in   dto.TimeZoneInfo
		want string
	}{
		{
			name: "registered timezone is returned as is",
			in:   dto.TimeZoneInfo{Name: "Europe/Moscow", Offset: 3 * time.Hour},
			want: "Europe/Moscow",
		},
		{
			name: "unknown timezone is resolved by its offset",
			in:   dto.TimeZoneInfo{Name: "Asia/Unknown", Offset: tokyoOffset, IsDST: tokyoIsDST},
			// имя сверяется со списком, а не с литералом: при совпадении смещений
			// выигрывает пояс, указанный в списке последним
			want: mustNameByOffset(t, locations, tokyoOffset, tokyoIsDST),
		},
		{
			name: "unresolvable timezone falls back to utc",
			in:   dto.TimeZoneInfo{Name: "Mars/Olympus", Offset: 1234 * time.Second}, // смещения нет ни у одного пояса
			want: "UTC",
		},
		{
			name: "empty request falls back to utc",
			in:   dto.TimeZoneInfo{},
			want: "UTC",
		},
		{
			// пояс процесса зарегистрирован в списке, но IANA-именем не является,
			// поэтому по имени не отдаётся; подбор идёт по смещению
			name: "process timezone is not accepted by name",
			in:   dto.TimeZoneInfo{Name: "Local", Offset: tokyoOffset, IsDST: tokyoIsDST},
			want: mustNameByOffset(t, locations, tokyoOffset, tokyoIsDST),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, servicetimezone.New(locations).Resolve(tt.in))
		})
	}
}

// mustNameByOffset - возвращает имя пояса, подобранное списком по смещению.
func mustNameByOffset(t *testing.T, locations *timezone.LocationList, offset time.Duration, isDST bool) string {
	t.Helper()

	name, ok := locations.NameByOffset(offset, isDST)
	require.True(t, ok)

	return name
}
