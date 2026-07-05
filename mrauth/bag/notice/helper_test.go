package notice_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mondegor/go-components/mrauth/bag/notice"
)

func TestKeyByEventAndRealm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event string
		realm string
		want  string
	}{
		{
			name:  "path realm",
			event: "user.authorization.success",
			realm: "site/admin",
			want:  "user.authorization.success.site.admin",
		},
		{
			name:  "realm without slash",
			event: "user.registration.success",
			realm: "admin",
			want:  "user.registration.success.admin",
		},
		{
			name:  "multiple slashes",
			event: "evt",
			realm: "a/b/c",
			want:  "evt.a.b.c",
		},
		{
			name:  "empty realm",
			event: "evt",
			realm: "",
			want:  "evt.",
		},
		{
			name:  "empty event",
			event: "",
			realm: "site/admin",
			want:  ".site.admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := notice.KeyByEventAndRealm(tt.event, tt.realm)
			assert.Equal(t, tt.want, got)
		})
	}
}
