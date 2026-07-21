package validate_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/validate"
)

func TestValidateLang(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "language and region", value: "ru-RU", want: true},
		{name: "language only", value: "en", want: true},

		// запись неканонична, но резолвер приводит её к поддерживаемому языку сам,
		// поэтому на границе ввода такие значения не отвергаются
		{name: "region in lower case", value: "ru-ru", want: true},
		{name: "language in upper case", value: "RU-RU", want: true},
		{name: "underscore instead of hyphen", value: "ru_RU", want: true},
		{name: "three letter language", value: "rus", want: true},

		{name: "empty", value: "", want: false},
		{name: "region without language", value: "-RU", want: false},
		{name: "trailing hyphen", value: "ru-", want: false},
		{name: "with space", value: "ru-RU ", want: false},
		{name: "with digits", value: "r1-RU", want: false},
		{name: "markup injection", value: "<>", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, validate.Lang(tt.value))
		})
	}
}
