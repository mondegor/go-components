package config_test

import (
	"testing"

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
