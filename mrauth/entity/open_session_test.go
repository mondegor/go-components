package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mondegor/go-components/mrauth/entity"
)

func TestOpenSessions_IDs(t *testing.T) {
	t.Parallel()

	at := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		list entity.OpenSessions
		want []uint32
	}{
		{
			name: "nil список - пустой результат",
			list: nil,
			want: []uint32{},
		},
		{
			name: "пустой список - пустой результат",
			list: entity.OpenSessions{},
			want: []uint32{},
		},
		{
			name: "порядок идентификаторов сохраняется",
			list: entity.OpenSessions{
				{SessionID: 30, ExpiresAt: at},
				{SessionID: 10, ExpiresAt: at},
				{SessionID: 20, ExpiresAt: at},
			},
			want: []uint32{30, 10, 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.list.IDs())
		})
	}
}

func TestOpenSessions_ExpiresAt(t *testing.T) {
	t.Parallel()

	first := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	second := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

	list := entity.OpenSessions{
		{SessionID: 10, ExpiresAt: first},
		{SessionID: 20, ExpiresAt: second},
	}

	tests := []struct {
		name      string
		list      entity.OpenSessions
		sessionID uint32
		want      time.Time
	}{
		{
			name:      "срок нужной сессии среди нескольких",
			list:      list,
			sessionID: 20,
			want:      second,
		},
		{
			name:      "сессия не найдена - нулевое время",
			list:      list,
			sessionID: 99,
			want:      time.Time{},
		},
		{
			name:      "пустой список - нулевое время",
			list:      entity.OpenSessions{},
			sessionID: 10,
			want:      time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.list.ExpiresAt(tt.sessionID))
		})
	}
}
