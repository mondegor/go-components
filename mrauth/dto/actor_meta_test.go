package dto_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/assert"

	"github.com/mondegor/go-components/mrauth/dto"
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
