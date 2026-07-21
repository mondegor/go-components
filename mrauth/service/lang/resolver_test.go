package lang_test

import (
	"testing"

	"github.com/mondegor/go-core/mrlocale"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	servicelang "github.com/mondegor/go-components/mrauth/service/lang"
)

type (
	// stubProvider - провайдер без переводов: резолверу важен только выбор языка.
	//
	// Это не мок коллаборатора, а фикстура для сборки настоящего mrlocale.Pool:
	// провайдер передаётся в mrlocale.NewBundle, а не в тестируемый резолвер, поэтому
	// правило "моки только через mockgen" здесь неприменимо - подменять нечего.
	stubProvider struct{}
)

func (p stubProvider) Domains() []string {
	return []string{mrlocale.DefaultMessagesDomain, mrlocale.DefaultErrorsDomain}
}

func (p stubProvider) Localize(_ string, _ language.Tag, msg string, _ []any) string {
	return msg
}

// newPool - создаёт пул на языках [ru-RU, en-US] с языком по умолчанию ru-RU,
// как это задаётся в приложении.
func newPool(t *testing.T) *mrlocale.Pool {
	t.Helper()

	bundle, err := mrlocale.NewBundle(
		[]string{"ru-RU", "en-US"},
		mrlocale.WithMessageProvider(func(_ []language.Tag) (mrlocale.MessageProvider, error) {
			return stubProvider{}, nil
		}),
	)
	require.NoError(t, err)

	return mrlocale.NewPool(bundle)
}

// TestResolver_Resolve - проверяет приведение языка к поддерживаемому приложением.
func TestResolver_Resolve(t *testing.T) {
	t.Parallel()

	resolver := servicelang.New(newPool(t))

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "supported regional language is returned as is",
			in:   "ru-RU",
			want: "ru-RU",
		},
		{
			name: "language without region is normalized to the regional one",
			in:   "ru",
			want: "ru-RU",
		},
		{
			// подчёркивание - разделитель, который порождает генератор gotext
			name: "underscore separated language is the same language",
			in:   "ru_RU",
			want: "ru-RU",
		},
		{
			name: "another region of the supported language",
			in:   "ru-UA",
			want: "ru-RU",
		},
		{
			name: "another supported language",
			in:   "en",
			want: "en-US",
		},
		{
			name: "unsupported language falls back to the default",
			in:   "de-DE",
			want: "ru-RU",
		},
		{
			name: "malformed language falls back to the default",
			in:   "not-a-language-tag!!!",
			want: "ru-RU",
		},
		{
			name: "empty language falls back to the default",
			in:   "",
			want: "ru-RU",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, resolver.Resolve(tc.in))
		})
	}
}
