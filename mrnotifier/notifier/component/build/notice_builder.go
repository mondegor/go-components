package build

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/mondegor/go-webcore/mrcore"

	"github.com/mondegor/go-components/mrmailer/dto"
	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrnotifier/template"
)

const (
	defaultChannelPrefix = "notifier"
	channelEmail         = "email"
	channelSMS           = "sms"
	channelMessenger     = "messenger"
)

type (
	// NoticeBuilder - собирает уведомление в сообщение/сообщения
	// с использованием шаблона для отправки получателям.
	NoticeBuilder struct {
		useCaseTemplate template.UseCase
		channelPrefix   string
	}
)

// New - создаёт объект NoticeBuilder.
func New(useCaseTemplate template.UseCase, opts ...Option) *NoticeBuilder {
	co := &NoticeBuilder{
		useCaseTemplate: useCaseTemplate,
		channelPrefix:   defaultChannelPrefix,
	}

	for _, opt := range opts {
		opt(co)
	}

	return co
}

// BuildNotice - сборка уведомления на основе шаблона и указанных переменных.
func (co *NoticeBuilder) BuildNotice(ctx context.Context, notice entity.Notice) (messages []dto.Message, err error) {
	templ, err := co.useCaseTemplate.GetItemByKey(ctx, notice.Key, notice.Data[mrnotifier.HeaderLang])
	if err != nil {
		return nil, co.wrapError(err, notice)
	}

	// если значение переменной в уведомлении явно не указано, то оно берётся из шаблона
	for _, v := range templ.Vars {
		if _, ok := notice.Data[v.Name]; !ok {
			notice.Data[v.Name] = v.DefaultValue
		}
	}

	messages = make([]dto.Message, 0, 4)

	if templ.Props.Email != nil && !templ.Props.Email.IsDisabled {
		list, err := co.buildEmail(notice.Data, templ.Props.Email)
		if err != nil {
			return nil, co.wrapError(err, notice)
		}

		messages = append(messages, list...)
	}

	if templ.Props.SMS != nil && !templ.Props.SMS.IsDisabled {
		list, err := co.buildSMS(notice.Data, templ.Props.SMS)
		if err != nil {
			return nil, co.wrapError(err, notice)
		}

		messages = append(messages, list...)
	}

	if templ.Props.Messenger != nil && !templ.Props.Messenger.IsDisabled {
		list, err := co.buildMessenger(notice.Data, templ.Props.Messenger)
		if err != nil {
			return nil, co.wrapError(err, notice)
		}

		messages = append(messages, list...)
	}

	// значение рассчитывается как можно позже, чтобы уменьшить погрешность
	sendAfter, err := co.getSendAfter(notice)
	if err != nil {
		return nil, co.wrapError(err, notice)
	}

	if len(messages) == 0 {
		return nil, co.wrapError(errors.New("notice is not built, no providers"), notice)
	}

	header := co.header(notice.Data)

	for i := range messages {
		messages[i].Channel += "/" + co.channelPrefix + "/" + notice.Key + "/" + templ.Lang
		messages[i].SendAfter = sendAfter
		messages[i].Data.Header = header
	}

	return messages, nil
}

func (co *NoticeBuilder) header(vars map[string]string) (header map[string]string) {
	for key, val := range vars {
		if cleanKey, ok := strings.CutPrefix(key, mrnotifier.HeaderPrefix); ok {
			if header == nil {
				header = make(map[string]string)
			}

			header[cleanKey] = val
		}
	}

	return header
}

func (co *NoticeBuilder) getSendAfter(notice entity.Notice) (time.Time, error) {
	if v := notice.Data[mrnotifier.ConfigDelayTime]; v != "" {
		// если указано числовое значение, то это продолжительность в секундах
		if delayPeriod, err := strconv.ParseUint(v, 10, 64); err == nil {
			return time.Now().Add(time.Duration(delayPeriod) * time.Second), nil
		}

		// если это число + unit, то это продолжительность
		if delayPeriod, err := time.ParseDuration(v); err == nil {
			return time.Now().Add(delayPeriod), nil
		}

		// если это время в формате RFC3339
		delayTime, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, mrcore.ErrUseCaseIncorrectInputData.Wrap(
				err,
				"notice.Data["+mrnotifier.ConfigDelayTime+"]",
				notice.Data[mrnotifier.ConfigDelayTime],
			)
		}

		if time.Until(delayTime) > 0 {
			return delayTime, nil
		}
	}

	return time.Time{}, nil
}

func (co *NoticeBuilder) wrapError(err error, notice entity.Notice) error {
	return mrcore.ErrUseCaseOperationFailed.Wrap(err).WithAttr("noticeKey", notice.Key)
}
