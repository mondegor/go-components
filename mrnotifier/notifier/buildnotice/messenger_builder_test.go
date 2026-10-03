package buildnotice_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

// TestMessengerBuilderRenderMode - сообщение мессенджера рендерится без HTML экранирования значений.
func TestMessengerBuilderRenderMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		value       string
		wantContent string
	}{
		{
			name:        "keeps email",
			value:       "user+tag@x.com",
			wantContent: "*Hi*\nHello, user+tag@x.com",
		},
		{
			name:        "keeps special chars",
			value:       `'"<>&`,
			wantContent: "*Hi*\nHello, '\"<>&",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			notices, err := newBuildManager().Build(
				map[string]string{"value": tt.value},
				entity.TemplateData{
					Messenger: &entity.DataMessenger{
						ChatID:  "chat",
						Subject: "Hi",
						Content: "Hello, {{.value}}",
					},
				},
			)
			require.NoError(t, err)
			require.Len(t, notices, 1)
			require.NotNil(t, notices[0].Data.Messenger)

			assert.Equal(t, "chat", notices[0].Data.Messenger.ChatID)
			assert.Equal(t, tt.wantContent, notices[0].Data.Messenger.Content)
		})
	}
}
