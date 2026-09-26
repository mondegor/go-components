package config_test

// база часовых поясов встраивается в тестовый бинарник, чтобы тесты проходили
// в минимальных образах, где она отсутствует в системе: без неё проверка
// существования пояса отвергла бы все IANA-имена.
import (
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/mondegor/go-core/util/crypt/password"
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

// TestValidateLanguages - язык, не помещающийся в колонку lang_code, отвергается
// при загрузке конфигурации, а не при первой записи в БД. Для самого mrlocale такие языки
// законны, поэтому отсеять их может только эта проверка.
//
// Общие требования к списку (разбор, каноничность) проверяются не здесь, а в
// mrlocale/config.ValidateLanguages; тут достаточно убедиться, что она вызывается.
func TestValidateLanguages(t *testing.T) {
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

			err := config.ValidateLanguages(tc.langs)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateTimeZones - имя пояса, не помещающееся в колонку user_timezone,
// отвергается при загрузке конфигурации. Для самого timezone.LocationList длина имени
// безразлична, поэтому отсеять его может только эта проверка.
//
// Общие требования к списку (непустота, уникальность, существование пояса) проверяются
// не здесь, а в util/timezone/config.ValidateTimeZones; тут достаточно убедиться,
// что она вызывается.
func TestValidateTimeZones(t *testing.T) {
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

			err := config.ValidateTimeZones(tc.zones)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestCorrectValuesAuth2FADecoyTOTPPercent - доля подставного TOTP принимается только в 1..99.
// Края исключены: и 0, и 100 раздали бы один и тот же подставной тип всем аккаунтам без 2FA,
// а вырожденное распределение выдаёт заглушку не хуже расхождения с реальным. Значение вне
// диапазона заменяется дефолтом здесь, а не роняет старт в crypt.NewDecoyFactorSelector.
func TestCorrectValuesAuth2FADecoyTOTPPercent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   uint8
		want uint8
	}{
		{name: "not set - default", in: 0, want: 50},
		{name: "lower bound is kept", in: 1, want: 1},
		{name: "value in range is kept", in: 17, want: 17},
		{name: "upper bound is kept", in: 99, want: 99},
		{name: "degenerate 100 - default", in: 100, want: 50},
		{name: "out of range - default", in: 200, want: 50},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := config.CorrectValuesAuth2FA(config.Auth2FA{DecoyTOTPPercent: tc.in})

			require.Equal(t, tc.want, got.DecoyTOTPPercent)
		})
	}
}

// TestValidateAuth2FADecoyFactorSalt - соль подставного второго фактора не кламплется, а
// отвергается: это секрет инсталляции, подставить ему умолчание нечем. Короткая соль
// перебирается, а подобравший её вычисляет ожидаемый подставной тип и по расхождению читает,
// включена ли у аккаунта 2FA - ровно то, что подстановка и скрывает.
func TestValidateAuth2FADecoyFactorSalt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		salt    string
		wantErr bool
	}{
		{name: "empty", salt: "", wantErr: true},
		{name: "one byte below min", salt: strings.Repeat("s", 31), wantErr: true},
		{name: "exactly min", salt: strings.Repeat("s", 32), wantErr: false},
		{name: "above min", salt: strings.Repeat("s", 64), wantErr: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateAuth2FA(config.Auth2FA{DecoyFactorSalt: tc.salt})

			if tc.wantErr {
				require.Error(t, err)
				// сама соль в сообщение не попадает: логи ошибок старта обычно не защищены
				require.NotContains(t, err.Error(), strings.Repeat("s", 8))

				return
			}

			require.NoError(t, err)
		})
	}
}

// TestCorrectValuesAuth2FAConfirmExpiry - срок жизни звена второго фактора ограничен сверху:
// SecureOperation.ActivateConfirmation переставляет ExpiresAt от текущего звена на каждом шаге,
// поэтому сроки звеньев складываются, и без потолка операция жила бы сколь угодно долго.
func TestCorrectValuesAuth2FAConfirmExpiry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{name: "not set - default", in: 0, want: 30 * time.Minute},
		{name: "negative - default", in: -time.Minute, want: 30 * time.Minute},
		{name: "value in range is kept", in: 5 * time.Minute, want: 5 * time.Minute},
		{name: "upper bound is kept", in: time.Hour, want: time.Hour},
		{name: "above upper bound - default", in: 24 * time.Hour, want: 30 * time.Minute},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := config.CorrectValuesAuth2FA(config.Auth2FA{ConfirmExpiry: tc.in})

			require.Equal(t, tc.want, got.ConfirmExpiry)
		})
	}
}

// TestParsePasswordStrength - порог надёжности пароля 2FA задаётся именем уровня из той же
// шкалы, что отдаёт POST /v1/check/calc-password-strength. NOT_RATED порогом быть не может:
// пароль без оценки недопустим как второй фактор, поэтому такое имя - ошибка конфигурации.
func TestParsePasswordStrength(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      string
		want    password.PassStrength
		wantErr bool
	}{
		{name: "weak", in: "WEAK", want: password.PassStrengthWeak},
		{name: "middle", in: "MIDDLE", want: password.PassStrengthMedium},
		{name: "strong", in: "STRONG", want: password.PassStrengthStrong},
		{name: "the best", in: "THE_BEST", want: password.PassStrengthBest},
		{name: "not rated", in: "NOT_RATED", wantErr: true},
		{name: "lowercase", in: "strong", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := config.ParsePasswordStrength(tc.in)

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestCorrectValuesAuth2FAPasswordMinStrength - незаданный порог заменяется умолчанием,
// заданный сохраняется как есть (его допустимость проверяет ValidateAuth2FA).
func TestCorrectValuesAuth2FAPasswordMinStrength(t *testing.T) {
	t.Parallel()

	require.Equal(t, "STRONG", config.CorrectValuesAuth2FA(config.Auth2FA{}).PasswordMinStrength)
	require.Equal(t, "MIDDLE", config.CorrectValuesAuth2FA(config.Auth2FA{PasswordMinStrength: "MIDDLE"}).PasswordMinStrength)
}

// TestValidateAuth2FAPasswordMinStrength - неизвестное имя порога роняет старт, а не молча
// заменяется умолчанием; незаданный порог допустим (его подставит CorrectValuesAuth2FA).
func TestValidateAuth2FAPasswordMinStrength(t *testing.T) {
	t.Parallel()

	salt := strings.Repeat("s", 32)

	require.NoError(t, config.ValidateAuth2FA(config.Auth2FA{DecoyFactorSalt: salt}))
	require.NoError(t, config.ValidateAuth2FA(config.Auth2FA{DecoyFactorSalt: salt, PasswordMinStrength: "THE_BEST"}))
	require.Error(t, config.ValidateAuth2FA(config.Auth2FA{DecoyFactorSalt: salt, PasswordMinStrength: "NOT_RATED"}))
	require.Error(t, config.ValidateAuth2FA(config.Auth2FA{DecoyFactorSalt: salt, PasswordMinStrength: "HARD"}))
}

// TestValidateAuth2FARecoveryCodeLength - длина аварийного кода вне диапазона, который принимает
// auth2fa.Verifier (и задаёт спека), роняет старт; незаданная длина допустима
// (её подставит CorrectValuesAuth2FA).
func TestValidateAuth2FARecoveryCodeLength(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		length  uint8
		wantErr bool
	}{
		{name: "not set", length: 0, wantErr: false},
		{name: "below min", length: 7, wantErr: true},
		{name: "exactly min", length: 8, wantErr: false},
		{name: "exactly max", length: 32, wantErr: false},
		{name: "above max", length: 33, wantErr: true},
	}

	salt := strings.Repeat("s", 32)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateAuth2FA(config.Auth2FA{DecoyFactorSalt: salt, RecoveryCodeLength: tc.length})

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

// TestValidateOperationConfirm - длина кода подтверждения ограничена диапазоном из спеки,
// а срок операции подтверждения нового емаила должен быть больше порога
// фиксированного срока: иначе повторная отправка кода продлевала бы его.
func TestValidateOperationConfirm(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cfg     config.OperationConfirm
		wantErr bool
	}{
		{name: "code length not set", cfg: config.OperationConfirm{}, wantErr: true},
		{name: "code length below min", cfg: config.OperationConfirm{CodeLength: 3}, wantErr: true},
		{name: "code length exactly min", cfg: config.OperationConfirm{CodeLength: 4}, wantErr: false},
		{name: "code length exactly max", cfg: config.OperationConfirm{CodeLength: 8}, wantErr: false},
		{name: "code length above max", cfg: config.OperationConfirm{CodeLength: 9}, wantErr: true},
		{name: "new email expiry not set", cfg: config.OperationConfirm{CodeLength: 6}, wantErr: false},
		{name: "new email expiry below threshold", cfg: config.OperationConfirm{CodeLength: 6, NewEmailExpiry: 30 * time.Minute}, wantErr: true},
		{name: "new email expiry equals threshold", cfg: config.OperationConfirm{CodeLength: 6, NewEmailExpiry: time.Hour}, wantErr: true},
		{name: "new email expiry above threshold", cfg: config.OperationConfirm{CodeLength: 6, NewEmailExpiry: 72 * time.Hour}, wantErr: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateOperationConfirm(tc.cfg)

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

// TestValidateRealmsConfirmCodeLength - заданная длина кода подтверждения realm'а проверяется
// так же, как у настроек по умолчанию; незаданная допустима (её подставит CorrectValuesRealm).
func TestValidateRealmsConfirmCodeLength(t *testing.T) {
	t.Parallel()

	makeRealms := func(codeLength uint8) []config.UserRealm {
		return []config.UserRealm{
			{
				ID:               1,
				Name:             "site",
				RegisterUserKind: "user",
				AuthToken:        config.Token{AccessType: "jwt"},
				OperationConfirm: config.OperationConfirm{CodeLength: codeLength},
				UserKinds: []config.UserKind{
					{Name: "user", Roles: []string{"guests"}},
				},
			},
		}
	}

	require.NoError(t, config.ValidateRealms(makeRealms(0), []string{"guests"}))
	require.NoError(t, config.ValidateRealms(makeRealms(6), []string{"guests"}))
	require.ErrorContains(t, config.ValidateRealms(makeRealms(9), []string{"guests"}), "confirm code length is out of range")
}
