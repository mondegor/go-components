package entity_test

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/assert"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
)

func TestNewSecurityLogEvent(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	ip := mrtype.NewIP(netip.MustParseAddr("192.0.2.1"))
	extra := &entity.SecurityLogExtra{Factor: "TOTP"}

	got := entity.NewSecurityLogEvent(userID, ip, "Mozilla/5.0", securityevent.Auth2FAEnabled, extra)

	assert.Equal(t, userID, got.UserID)
	assert.Equal(t, ip, got.ClientIP)
	assert.Equal(t, "Mozilla/5.0", got.UserAgent)
	assert.Equal(t, securityevent.Auth2FAEnabled, got.EventType)
	assert.Same(t, extra, got.Extra)
	// время события задаёт хранилище при записи
	assert.True(t, got.CreatedAt.IsZero())
}
