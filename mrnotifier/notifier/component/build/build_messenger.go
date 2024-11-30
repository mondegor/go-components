package build

import (
	"regexp"
	"strings"

	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"

	"github.com/mondegor/go-components/mrmailer/dto"
	templaterentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

var (
	regexpMessengerTag       = regexp.MustCompile(`^@[0-9A-Za-z_]+$`)
	replacerMessengerSubject = strings.NewReplacer( //nolint:gochecknoglobals
		"_", "\\_",
		"*", "\\*",
	)
)

func (co *NoticeBuilder) buildMessenger(vars map[string]string, messenger *templaterentity.DataMessenger) ([]dto.Message, error) {
	if messenger.Content == "" {
		return nil, mrcore.ErrInternalWithDetails.New("messenger.Content is empty")
	}

	var contentBilder strings.Builder

	for i, tag := range messenger.Tags {
		if !regexpMessengerTag.MatchString(tag) {
			return nil, mrcore.ErrInternalValueNotMatchRegexpPattern.New(tag, regexpMessengerTag.String())
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

	content := contentBilder.String()

	content, err := mrmsg.Render(content, vars)
	if err != nil {
		return nil, mrcore.ErrInternalWithDetails.Wrap(err, "content rendering failed")
	}

	return []dto.Message{
		{
			Channel: channelMessenger,
			Data: dto.MessageData{
				Messenger: &dto.DataMessenger{
					ChatID:  messenger.ChatID,
					Content: content,
				},
			},
		},
	}, nil
}
