package dto_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/assert"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
)

func TestActorMeta_Location(t *testing.T) {
	t.Parallel()

	msk := time.FixedZone("MSK", 3*60*60)

	tests := []struct {
		name  string
		actor dto.ActorMeta
		want  *time.Location
	}{
		{
			name:  "user location",
			actor: dto.NewAnonymousActorMeta(mrtype.DetailedIP{}, "", msk),
			want:  msk,
		},
		{
			name:  "nil location is UTC",
			actor: dto.NewAnonymousActorMeta(mrtype.DetailedIP{}, "", nil),
			want:  time.UTC,
		},
		{
			name:  "without constructor is UTC",
			actor: dto.ActorMeta{},
			want:  time.UTC,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.actor.Location())
		})
	}
}

func TestActorMeta_WithUserKeepsLocation(t *testing.T) {
	t.Parallel()

	msk := time.FixedZone("MSK", 3*60*60)
	actor := dto.NewAnonymousActorMeta(mrtype.DetailedIP{}, "", msk).WithUser(uuid.New())

	assert.Equal(t, msk, actor.Location())
}

func TestNewAnonymousActorMeta(t *testing.T) {
	t.Parallel()

	actor := dto.NewAnonymousActorMeta(mrtype.DetailedIP{}, "test-agent", nil)

	assert.Equal(t, uuid.Nil, actor.UserID)
	assert.Equal(t, "test-agent", actor.UserAgent)
	assert.Equal(t, time.UTC, actor.Location())
}

// TestActorMeta_NewSecurityEvent - запись журнала безопасности получает пользователя,
// IP и user agent актора.
func TestActorMeta_NewSecurityEvent(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	ip := mrtype.NewIP(netip.MustParseAddr("192.0.2.1"))
	extra := &entity.SecurityLogExtra{NewValue: "new@example.com"}

	got := dto.NewActorMeta(userID, ip, "test-agent", nil).NewSecurityEvent(securityevent.EmailChangeRequested, extra)

	assert.Equal(t, userID, got.UserID)
	assert.Equal(t, ip, got.ClientIP)
	assert.Equal(t, "test-agent", got.UserAgent)
	assert.Equal(t, securityevent.EmailChangeRequested, got.EventType)
	assert.Same(t, extra, got.Extra)
}
