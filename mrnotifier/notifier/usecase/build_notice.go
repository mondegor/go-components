package usecase

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrmsg/templater"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/buildnotice"
	"github.com/mondegor/go-components/mrnotifier/notifier/dto"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	templatedto "github.com/mondegor/go-components/mrnotifier/template/dto"
	templateentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

const (
	defaultChannelPrefix = "notifier"
)

type (
	// BuildNotice - собирает уведомление в сообщение/сообщения
	// с использованием шаблона для отправки получателям.
	BuildNotice struct {
		serviceTemplate templateService
		noticeBuilder   noticeBuilder
		channelPrefix   string
		errorWrapper    errors.Wrapper
	}

	templateService interface {
		GetItemByKey(ctx context.Context, name, lang string) (templatedto.Template, error)
	}

	noticeBuilder interface {
		Build(vars map[string]string, templ templateentity.TemplateData) ([]dto.Notice, error)
	}
)

// New - создаёт объект BuildNotice.
func New(
	serviceTemplate templateService,
	opts ...Option,
) *BuildNotice {
	o := options{
		builder: &BuildNotice{
			serviceTemplate: serviceTemplate,
			noticeBuilder:   buildnotice.NewBuildManager(templater.NewTemplater("{{", "}}")),
			channelPrefix:   defaultChannelPrefix,
			errorWrapper:    errors.NewUseCaseWrapper(),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o.builder
}

// Execute - сборка уведомления на основе шаблона и указанных переменных.
func (uc *BuildNotice) Execute(ctx context.Context, note entity.Note) (notices []dto.Notice, err error) {
	templ, err := uc.serviceTemplate.GetItemByKey(ctx, note.Key, note.Data[mrnotifier.HeaderLang])
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err, "noticeKey", note.Key)
	}

	// если значение переменной в уведомлении явно не указано, то оно берётся из шаблона
	for _, v := range templ.Vars {
		if _, ok := note.Data[v.Name]; !ok {
			note.Data[v.Name] = v.DefaultValue
		}
	}

	notices, err = uc.noticeBuilder.Build(note.Data, templ.Props)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err, "noticeKey", note.Key)
	}

	// значение рассчитывается как можно позже, чтобы уменьшить погрешность
	sendAfter, err := uc.getSendAfter(note)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err, "noticeKey", note.Key)
	}

	if len(notices) == 0 {
		return nil, errors.NewInternalError("notice is not built, no providers", "noticeKey", note.Key)
	}

	header := uc.header(note.Data)

	for i := range notices {
		notices[i].Channel += "/" + uc.channelPrefix + "/" + note.Key + "/" + templ.Lang
		notices[i].SendAfter = sendAfter
		notices[i].Data.Header = header
	}

	return notices, nil
}

func (uc *BuildNotice) header(vars map[string]string) (header map[string]string) {
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

func (uc *BuildNotice) getSendAfter(notice entity.Note) (time.Time, error) {
	if v := notice.Data[mrnotifier.ConfigDelayTime]; v != "" {
		// если указано числовое значение, то это продолжительность в секундах
		if delayPeriod, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Now().Add(time.Duration(delayPeriod) * time.Second), nil
		}

		// если это число + unit, то это продолжительность
		if delayPeriod, err := time.ParseDuration(v); err == nil {
			return time.Now().Add(delayPeriod), nil
		}

		// если это время в формате RFC3339
		delayTime, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, errors.ErrInternalIncorrectInputData.WithError(
				err,
				"BuildNotice",
				"noticeDataKey", mrnotifier.ConfigDelayTime,
				"noticeDataItem", v,
			)
		}

		if time.Until(delayTime) > 0 {
			return delayTime, nil
		}
	}

	return time.Time{}, nil
}
