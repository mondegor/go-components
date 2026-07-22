package contactaddress_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/model/contactaddress"
)

func TestValidateEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "simple", value: "user@example.com", want: true},
		{name: "dots and plus in local part", value: "user.name+tag@sub.example.co.uk", want: true},
		{name: "short", value: "a@b.co", want: true},
		{name: "hyphen inside domain label", value: "user@my-example.com", want: true},

		{name: "empty", value: "", want: false},
		{name: "without at", value: "user.example.com", want: false},
		{name: "without domain", value: "user@", want: false},
		{name: "without top level domain", value: "user@example", want: false},
		{name: "empty domain label", value: "user@..com", want: false},
		{name: "domain label is hyphen", value: "user@-.com", want: false},
		{name: "trailing hyphen in domain label", value: "user@example-.com", want: false},
		{name: "leading dot in local part", value: ".user@example.com", want: false},
		{name: "empty segment in local part", value: "user..name@example.com", want: false},
		{name: "underscore in domain", value: "user@ex_ample.com", want: false},
		{name: "with space", value: "user@example.com ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, contactaddress.ValidateEmail(tt.value))
		})
	}
}

func TestValidatePhone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "with plus", value: "+79001234567", want: true},
		{name: "with separators", value: "8 (900) 123-45-67", want: true},
		{name: "digits only", value: "79001234567", want: true},

		{name: "empty", value: "", want: false},
		{name: "single digit", value: "7", want: false},
		{name: "with letters", value: "+7900123456a", want: false},
		{name: "space after plus", value: "+ 79001234567", want: false},
		{name: "trailing separator", value: "+7 900 123 45 67-", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, contactaddress.ValidatePhone(tt.value))
		})
	}
}

func TestValidatePhoneCIS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "russian with plus", value: "+79001234567", want: true},
		{name: "russian with eight", value: "89001234567", want: true},
		{name: "russian without plus", value: "79001234567", want: true},
		{name: "neighbour country", value: "+9971234567", want: true},

		// значение должно совпадать со всей строкой целиком, а не с её началом или концом
		{name: "trailing garbage", value: "+79001234567abc", want: false},
		{name: "leading garbage", value: "garbage+9971234567", want: false},

		// номер формату соответствует, но принадлежит исключённому диапазону
		{name: "excluded prefix", value: "+79980001122", want: false},

		{name: "empty", value: "", want: false},
		{name: "too short", value: "+7900123456", want: false},
		{name: "unsupported country", value: "+14155552671", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, contactaddress.ValidatePhoneCIS(tt.value))
		})
	}
}

func TestValidatePhoneWorld(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "russian", value: "+79001234567", want: true},
		{name: "usa", value: "+14155552671", want: true},

		{name: "trailing garbage", value: "+14155552671abc", want: false},
		{name: "leading garbage", value: "garbage+14155552671", want: false},

		// номер формату соответствует, но принадлежит исключённому диапазону
		{name: "excluded prefix", value: "+79980001122", want: false},

		{name: "empty", value: "", want: false},
		{name: "too short", value: "+141555526", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, contactaddress.ValidatePhoneWorld(tt.value))
		})
	}
}
