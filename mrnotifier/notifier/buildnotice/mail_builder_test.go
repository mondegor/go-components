package buildnotice_test

import (
	"testing"

	"github.com/mondegor/go-core/mrmsg/templater"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrnotifier/notifier/buildnotice"
	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

// TestMailBuilderRenderMode - тема всегда рендерится без экранирования,
// тело экранируется как HTML только у писем типа text/html (без учёта параметров и регистра).
func TestMailBuilderRenderMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		value       string
		wantSubject string
		wantContent string
	}{
		{
			name:        "default type keeps email",
			contentType: "",
			value:       "user+tag@x.com",
			wantSubject: "Hi user+tag@x.com",
			wantContent: "Hello, user+tag@x.com",
		},
		{
			name:        "text plain keeps email",
			contentType: "text/plain",
			value:       "user+tag@x.com",
			wantSubject: "Hi user+tag@x.com",
			wantContent: "Hello, user+tag@x.com",
		},
		{
			name:        "text plain keeps special chars",
			contentType: "text/plain; charset=utf-8",
			value:       `'"<>&`,
			wantSubject: `Hi '"<>&`,
			wantContent: `Hello, '"<>&`,
		},
		{
			name:        "text html escapes content only",
			contentType: "text/html",
			value:       "<i>",
			wantSubject: "Hi <i>",
			wantContent: "Hello, &lt;i&gt;",
		},
		{
			name:        "text html with charset",
			contentType: "text/html; charset=utf-8",
			value:       "<i>",
			wantSubject: "Hi <i>",
			wantContent: "Hello, &lt;i&gt;",
		},
		{
			name:        "text html with charset in mixed case",
			contentType: "text/html; Charset=Utf-8",
			value:       "<i>",
			wantSubject: "Hi <i>",
			wantContent: "Hello, &lt;i&gt;",
		},
		{
			name:        "text html in upper case",
			contentType: "TEXT/HTML",
			value:       "<i>",
			wantSubject: "Hi <i>",
			wantContent: "Hello, &lt;i&gt;",
		},
		{
			name:        "text html with not utf-8 charset",
			contentType: "text/html; charset=koi8-r",
			value:       "<i>",
			wantSubject: "Hi <i>",
			wantContent: "Hello, &lt;i&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			notices, err := newBuildManager().Build(
				map[string]string{"value": tt.value},
				newMailTemplate(tt.contentType),
			)
			require.NoError(t, err)
			require.Len(t, notices, 1)
			require.NotNil(t, notices[0].Data.Mail)

			assert.Equal(t, tt.contentType, notices[0].Data.Mail.ContentType)
			assert.Equal(t, tt.wantSubject, notices[0].Data.Mail.Subject)
			assert.Equal(t, tt.wantContent, notices[0].Data.Mail.Content)
		})
	}
}

// TestMailBuilderInvalidContentType - нераспознаваемый тип письма является ошибкой, а не поводом молча выбрать режим.
func TestMailBuilderInvalidContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
	}{
		{
			name:        "invalid params",
			contentType: "text/html;;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := newBuildManager().Build(
				map[string]string{"value": "<i>"},
				newMailTemplate(tt.contentType),
			)
			require.Error(t, err)
		})
	}
}

func newBuildManager() *buildnotice.BuildManager {
	return buildnotice.NewBuildManager(
		templater.NewTemplater("{{", "}}"),
		templater.NewTemplater("{{", "}}", templater.WithMode(templater.ModeHTML)),
	)
}

func newMailTemplate(contentType string) entity.TemplateData {
	to := "to@x.com"

	return entity.TemplateData{
		Mail: &entity.DataMail{
			ContentType: contentType,
			To:          &to,
			Subject:     "Hi {{.value}}",
			Content:     "Hello, {{.value}}",
		},
	}
}
