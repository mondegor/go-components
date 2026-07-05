package config_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/wire/mrauth/config"
)

func TestResolveRefreshCookie(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }

	type testCase struct {
		name         string
		cfg          config.RefreshCookie
		wantErr      bool
		wantSecure   bool
		wantSameSite http.SameSite
		wantName     string
	}

	tests := []testCase{
		{
			name:         "safe defaults: secure=true, samesite=strict, name=RTID",
			cfg:          config.RefreshCookie{Domain: "example.com"},
			wantSecure:   true,
			wantSameSite: http.SameSiteStrictMode,
			wantName:     "RTID",
		},
		{
			name:    "domain is required",
			cfg:     config.RefreshCookie{},
			wantErr: true,
		},
		{
			name:         "explicit secure=false + samesite=lax allowed",
			cfg:          config.RefreshCookie{Domain: "example.com", Secure: boolPtr(false), SameSite: "lax"},
			wantSecure:   false,
			wantSameSite: http.SameSiteLaxMode,
			wantName:     "RTID",
		},
		{
			name:    "samesite=none without secure rejected",
			cfg:     config.RefreshCookie{Domain: "example.com", Secure: boolPtr(false), SameSite: "none"},
			wantErr: true,
		},
		{
			name:         "samesite=None (case-insensitive) with default secure allowed",
			cfg:          config.RefreshCookie{Domain: "example.com", SameSite: "None"},
			wantSecure:   true,
			wantSameSite: http.SameSiteNoneMode,
			wantName:     "RTID",
		},
		{
			name:    "invalid samesite rejected",
			cfg:     config.RefreshCookie{Domain: "example.com", SameSite: "bogus"},
			wantErr: true,
		},
		{
			name:         "custom name preserved",
			cfg:          config.RefreshCookie{Domain: "example.com", Name: "SID"},
			wantSecure:   true,
			wantSameSite: http.SameSiteStrictMode,
			wantName:     "SID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := config.ResolveRefreshCookie(tt.cfg)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantSecure, got.Secure)
			assert.Equal(t, tt.wantSameSite, got.SameSite)
			assert.Equal(t, tt.wantName, got.Name)
			assert.Equal(t, "/", got.Path)
			assert.Positive(t, got.Expiry)
		})
	}
}
