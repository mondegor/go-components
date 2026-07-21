package validate_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/validate"
)

func TestValidateTimeZone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "region and city", value: "Europe/Moscow", want: true},
		{name: "three parts", value: "America/Argentina/Salta", want: true},
		{name: "without slash", value: "UTC", want: true},
		{name: "with digits and sign", value: "Etc/GMT+5", want: true},
		{name: "with underscore", value: "America/New_York", want: true},
		{name: "with dot", value: "America/Port-au-Prince", want: true},

		// пояс процесса зависит от настроек хоста, поэтому клиенту не разрешён
		{name: "process timezone", value: "Local", want: false},
		{name: "empty", value: "", want: false},
		{name: "leading slash", value: "/Moscow", want: false},
		{name: "trailing slash", value: "Europe/", want: false},
		{name: "too many parts", value: "a/b/c/d", want: false},
		{name: "leading digit", value: "1Europe/Moscow", want: false},
		{name: "with space", value: "Europe/Moscow ", want: false},
		{name: "path traversal", value: "../../etc/passwd", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, validate.TimeZone(tt.value))
		})
	}
}
