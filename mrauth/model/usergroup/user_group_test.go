package usergroup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mondegor/go-components/mrauth/model/usergroup"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		realm string
		kind  string
		want  string
	}{
		{
			name:  "простой realm",
			realm: "site",
			kind:  "standard",
			want:  "site/standard",
		},
		{
			name:  "realm с разделителем внутри",
			realm: "site/admin",
			kind:  "manager",
			want:  "site/admin/manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, usergroup.Build(tt.realm, tt.kind))
		})
	}
}

func TestRealm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		group string
		want  string
	}{
		{
			name:  "вид пользователя отрезается",
			group: "site/standard",
			want:  "site",
		},
		{
			name:  "отрезается только по последнему разделителю",
			group: "site/admin/manager",
			want:  "site/admin",
		},
		{
			name:  "без разделителя группа целиком - это realm",
			group: "site",
			want:  "site",
		},
		{
			name:  "пустой вид пользователя",
			group: "site/",
			want:  "site",
		},
		{
			name:  "пустая группа",
			group: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, usergroup.Realm(tt.group))
		})
	}
}

// TestRealmOfNew - склейка и разбор парные: имя realm'а переживает round-trip, пока '/'
// есть только в realm'е. Это то самое ограничение, на которое опирается конфиг хоста
// (см. wire/mrauth/config.UserRealm).
func TestRealmOfNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		realm string
		kind  string
		want  string
	}{
		{
			name:  "realm без разделителя",
			realm: "site",
			kind:  "standard",
			want:  "site",
		},
		{
			name:  "realm с разделителем",
			realm: "site/admin",
			kind:  "manager",
			want:  "site/admin",
		},
		{
			name:  "пустой вид пользователя",
			realm: "site/admin",
			kind:  "",
			want:  "site/admin",
		},
		{
			name:  "'/' в виде пользователя ломает round-trip: realm разъезжается с конфигом",
			realm: "site",
			kind:  "admin/ro",
			want:  "site/admin", // а не "site" - именно поэтому '/' в kind запрещён
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, usergroup.Realm(usergroup.Build(tt.realm, tt.kind)))
		})
	}
}

// TestValidateKind - '/' в имени вида пользователя отвергается (он ломает round-trip
// Build/Realm, см. TestRealmOfNew), допустимые имена проходят без ошибки.
func TestValidateKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		kind    string
		wantErr bool
	}{
		{
			name: "обычное имя",
			kind: "standard",
		},
		{
			name: "пустое имя допустимо",
			kind: "",
		},
		{
			name:    "разделитель в имени отвергается",
			kind:    "admin/ro",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := usergroup.ValidateKind(tt.kind)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
