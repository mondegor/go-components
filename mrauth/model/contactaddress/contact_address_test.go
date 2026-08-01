package contactaddress_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
)

// TestNewValidAddress - конструктор получает значение, уже прошедшее проверку на границе ввода,
// поэтому телефон он обязан нормализовать так же, как разбор: иначе дальше по потоку уедет
// сырая строка, а DigitValue останется нулевым (поиск пользователя по телефону сломается).
func TestNewValidAddress(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		value          string
		wantKind       addresstype.Enum
		wantValue      string
		wantDigitValue uint64
	}

	tests := []testCase{
		{
			name:      "email in upper case",
			value:     "User@Example.COM",
			wantKind:  addresstype.Email,
			wantValue: "user@example.com",
		},
		{
			name:           "phone with separators",
			value:          "+7 (999) 123-45-67",
			wantKind:       addresstype.Phone,
			wantValue:      "79991234567",
			wantDigitValue: 79991234567,
		},
		{
			name:           "russian phone with eight",
			value:          "89991234567",
			wantKind:       addresstype.Phone,
			wantValue:      "79991234567",
			wantDigitValue: 79991234567,
		},

		// значения ниже отсекаются тегом на границе ввода, поэтому здесь они означают
		// нарушение инварианта: адрес остаётся пустым и потребитель вернёт явную ошибку
		{name: "phone is too short", value: "376123456"},
		{name: "phone of zeros", value: "0000000000"},
		{name: "not an address", value: "not-an-address"},
		{name: "empty", value: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			address := contactaddress.NewValidAddress(tt.value)

			require.Equal(t, tt.wantKind == addresstype.Email, address.Is(addresstype.Email))
			require.Equal(t, tt.wantKind == addresstype.Phone, address.Is(addresstype.Phone))
			require.Equal(t, tt.wantValue, address.Value())
			require.Equal(t, tt.wantDigitValue, address.DigitValue())
		})
	}
}

// TestNewEmail - email нормализуется только регистром, значение не проверяется:
// оно обязано быть проверено на границе ввода (ValidateEmail).
func TestNewEmail(t *testing.T) {
	t.Parallel()

	address := contactaddress.NewEmail("User@Example.COM")

	require.True(t, address.Is(addresstype.Email))
	require.Equal(t, "user@example.com", address.Value())
}

func TestNewDigitPhone(t *testing.T) {
	t.Parallel()

	address := contactaddress.NewDigitPhone(79991234567)

	require.True(t, address.Is(addresstype.Phone))
	require.Equal(t, "79991234567", address.Value())
	require.Equal(t, uint64(79991234567), address.DigitValue())
}
