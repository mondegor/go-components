package config_test

// база часовых поясов встраивается в тестовый бинарник, чтобы тесты проходили
// в минимальных образах, где она отсутствует в системе: без неё проверка
// существования пояса отвергла бы все IANA-имена.
import (
	"strings"
	"testing"
	_ "time/tzdata"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/wire/mrauth/config"
)

// TestValidateRealmsKindNameSeparator - '/' в имени вида пользователя отвергается на старте:
// он ломает разбор группы "{realm}/{kind}" и молча терял бы per-realm статистику этого вида
// (см. ограничение в описании config.UserRealm).
func TestValidateRealmsKindNameSeparator(t *testing.T) {
	t.Parallel()

	makeRealms := func(kindName string) []config.UserRealm {
		return []config.UserRealm{
			{
				ID:               1,
				Name:             "site/admin", // в имени realm'а '/' допустим
				RegisterUserKind: kindName,
				AuthToken:        config.Token{AccessType: "jwt"},
				UserKinds: []config.UserKind{
					{Name: kindName, Roles: []string{"guests"}},
				},
			},
		}
	}

	require.NoError(t, config.ValidateRealms(makeRealms("manager"), []string{"guests"}))
	require.ErrorContains(t, config.ValidateRealms(makeRealms("manager/ro"), []string{"guests"}), "must not contain separator")
}

func TestValidateSessionThresholds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		soft, hard int8
		wantErr    bool
	}{
		{name: "zeros mean no band", soft: 0, hard: 0, wantErr: false},
		{name: "negative within range", soft: -4, hard: -4, wantErr: false},
		{name: "below min rejected", soft: -5, hard: -5, wantErr: true},
		{name: "explicit valid", soft: 2, hard: 6, wantErr: false},
		{name: "hard below soft", soft: 5, hard: 1, wantErr: true},
		{name: "soft exceeds max", soft: 17, hard: 17, wantErr: true},
		{name: "hard exceeds max", soft: 0, hard: 17, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateSessionThresholds(tc.soft, tc.hard)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateStorableLanguages - язык, не помещающийся в колонку lang_code, отвергается
// при загрузке конфигурации, а не при первой записи в БД. Для самого mrlocale такие языки
// законны, поэтому отсеять их может только эта проверка.
//
// Общие требования к списку (разбор, каноничность) проверяются не здесь, а в
// mrlocale/config.ValidateLanguages; тут достаточно убедиться, что она вызывается.
func TestValidateStorableLanguages(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		langs   []string
		wantErr bool
	}{
		{name: "empty list is not rejected here", langs: nil, wantErr: false},
		{name: "language with region", langs: []string{"ru-RU", "en-US"}, wantErr: false},
		{name: "language without region", langs: []string{"ru", "en"}, wantErr: false},
		{name: "script subtag is rejected", langs: []string{"ru-RU", "zh-Hans"}, wantErr: true},
		{name: "three-letter code is rejected", langs: []string{"fil"}, wantErr: true},
		{name: "numeric region is rejected", langs: []string{"es-419"}, wantErr: true},
		// проверка из go-core доезжает: неканоничное и неразбираемое отвергаются ею
		{name: "underscore form is rejected", langs: []string{"en_US"}, wantErr: true},
		{name: "region in lower case is rejected", langs: []string{"ru-ru"}, wantErr: true},
		{name: "language in upper case is rejected", langs: []string{"RU-RU"}, wantErr: true},
		{name: "malformed name is rejected", langs: []string{"not-a-language-tag!!!"}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateStorableLanguages(tc.langs)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateStorableTimeZones - имя пояса, не помещающееся в колонку user_timezone,
// отвергается при загрузке конфигурации. Для самого timezone.LocationList длина имени
// безразлична, поэтому отсеять его может только эта проверка.
//
// Общие требования к списку (непустота, уникальность, существование пояса) проверяются
// не здесь, а в util/timezone/config.ValidateTimeZones; тут достаточно убедиться,
// что она вызывается.
func TestValidateStorableTimeZones(t *testing.T) {
	t.Parallel()

	// имя укладывается в формат IANA-имени и потому доходит до проверки длины,
	// но в колонку не помещается
	longName := "Europe/" + strings.Repeat("Moscow_", 9)
	require.Greater(t, len(longName), 64)

	cases := []struct {
		name    string
		zones   []string
		wantErr bool
	}{
		{name: "empty list is not rejected here", zones: nil, wantErr: false},
		{name: "region and city", zones: []string{"Europe/Moscow", "Asia/Tokyo"}, wantErr: false},
		{name: "longest real name", zones: []string{"America/Argentina/Buenos_Aires"}, wantErr: false},
		{name: "too long name is rejected", zones: []string{longName}, wantErr: true},
		// проверка из go-core доезжает: несуществующий пояс и "Local" отвергаются ею
		{name: "misspelled name is rejected", zones: []string{"Europe/Moscw"}, wantErr: true},
		{name: "process time zone is rejected", zones: []string{"Local"}, wantErr: true},
		{name: "empty name is rejected", zones: []string{""}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateStorableTimeZones(tc.zones)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
