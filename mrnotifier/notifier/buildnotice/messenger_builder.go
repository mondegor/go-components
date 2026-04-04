package buildnotice

import (
	"regexp"
	"strings"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrnotifier/notifier/dto"
	templateentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

type (
	// messengerBuilder - собирает уведомление/уведомления с использованием шаблона
	// в сообщение/сообщения мессенжера для отправки получателям.
	messengerBuilder struct {
		noticeRenderer noticeRenderer
		channel        string
	}
)

//nolint:gochecknoglobals
var (
	regexpMessengerTag       = regexp.MustCompile(`^@[0-9A-Za-z_]+$`)
	replacerMessengerSubject = strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
	)
	errInternalValueNotMatchRegexpPattern = errors.NewInternalProto("specified value does not match regexp pattern")
)

func newMessengerBuilder(
	noticeRenderer noticeRenderer,
	channel string,
) *messengerBuilder {
	return &messengerBuilder{
		noticeRenderer: noticeRenderer,
		channel:        channel,
	}
}

// Build - возвращает подготовленные уведомления на основе переданных переменных и данных из шаблона уведомления.
func (b *messengerBuilder) Build(vars map[string]string, messenger *templateentity.DataMessenger) ([]dto.Notice, error) {
	if messenger.Content == "" {
		return nil, errors.NewInternalError("messenger.Content is empty")
	}

	var contentBilder strings.Builder

	for i, tag := range messenger.Tags {
		if !regexpMessengerTag.MatchString(tag) {
			return nil, errInternalValueNotMatchRegexpPattern.New(
				"value", tag,
				"pattern", regexpMessengerTag.String(),
			)
		}

		if i > 0 {
			contentBilder.WriteByte(' ')
		}

		contentBilder.WriteString(tag)
	}

	if len(messenger.Tags) > 0 {
		contentBilder.WriteByte('\n')
	}

	if messenger.Subject != "" {
		contentBilder.WriteByte('*')
		replacerMessengerSubject.WriteString(&contentBilder, messenger.Subject) //nolint:errcheck
		contentBilder.WriteString("*\n")
	}

	contentBilder.WriteString(messenger.Content)

	content, err := b.noticeRenderer.Render(contentBilder.String(), vars) // TODO: временно
	if err != nil {
		return nil, errors.WrapInternalError(err, "content rendering failed")
	}

	return []dto.Notice{
		{
			Channel: b.channel,
			Data: dto.NoticeData{
				Messenger: &dto.DataMessenger{
					ChatID:  messenger.ChatID,
					Content: content,
				},
			},
		},
	}, nil
}
