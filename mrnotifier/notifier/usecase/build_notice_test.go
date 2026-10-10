package usecase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrnotifier/notifier/usecase"
	"github.com/mondegor/go-components/mrnotifier/notifier/usecase/mock"
	templatedto "github.com/mondegor/go-components/mrnotifier/template/dto"
	templateentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

//go:generate mockgen -source=build_notice.go -destination=mock/build_notice.go -package=mock

const (
	testNoticeKey = "user.registered"
	testLang      = "ru_RU"
)

// messengerTemplate - шаблон с единственным каналом messenger, контент которого выводит переменную name.
func messengerTemplate(vars ...templateentity.Variable) templatedto.Template {
	return templatedto.Template{
		Lang: testLang,
		Props: templateentity.TemplateData{
			Messenger: &templateentity.DataMessenger{
				ChatID:  "chat",
				Content: "Hello, {{.name}}",
			},
		},
		Vars: vars,
	}
}

func TestBuildNotice_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	serviceTemplate := mock.NewMocktemplateService(ctrl)
	serviceTemplate.EXPECT().GetItemByKey(gomock.Any(), testNoticeKey, testLang).Return(messengerTemplate(), nil)

	uc := usecase.New(serviceTemplate)

	notices, err := uc.Execute(context.Background(), entity.Note{
		Key: testNoticeKey,
		Data: map[string]string{
			"name":                     "Ivan",
			mrnotifier.HeaderLang:      testLang,
			"header.custom":            "value",
			mrnotifier.ConfigDelayTime: "",
		},
	})
	require.NoError(t, err)
	require.Len(t, notices, 1)
	require.NotNil(t, notices[0].Data.Messenger)

	assert.Equal(t, "messenger/notifier/"+testNoticeKey+"/"+testLang, notices[0].Channel)
	assert.Equal(t, "Hello, Ivan", notices[0].Data.Messenger.Content)
	assert.True(t, notices[0].SendAfter.IsZero())
	// в заголовок попадают только переменные с префиксом заголовка, без самого префикса
	assert.Equal(
		t,
		map[string]string{
			strings.TrimPrefix(mrnotifier.HeaderLang, mrnotifier.HeaderPrefix): testLang,
			"custom": "value",
		},
		notices[0].Data.Header,
	)
}

func TestBuildNotice_Execute_WithChannelPrefix(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	serviceTemplate := mock.NewMocktemplateService(ctrl)
	serviceTemplate.EXPECT().GetItemByKey(gomock.Any(), testNoticeKey, "").Return(messengerTemplate(), nil)

	uc := usecase.New(serviceTemplate, usecase.WithChannelPrefix("custom"))

	notices, err := uc.Execute(context.Background(), entity.Note{
		Key:  testNoticeKey,
		Data: map[string]string{"name": "Ivan"},
	})
	require.NoError(t, err)
	require.Len(t, notices, 1)

	assert.Equal(t, "messenger/custom/"+testNoticeKey+"/"+testLang, notices[0].Channel)
	assert.Nil(t, notices[0].Data.Header)
}

// TestBuildNotice_Execute_TemplateVars - значение по умолчанию из шаблона подставляется
// только для переменной, которая не указана в уведомлении (в т.ч. указанной пустой).
func TestBuildNotice_Execute_TemplateVars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		data        map[string]string
		wantContent string
	}{
		{name: "absent var takes default", data: map[string]string{}, wantContent: "Hello, Guest"},
		{name: "present var is kept", data: map[string]string{"name": "Ivan"}, wantContent: "Hello, Ivan"},
		{name: "empty var is kept", data: map[string]string{"name": ""}, wantContent: "Hello, "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			serviceTemplate := mock.NewMocktemplateService(ctrl)
			serviceTemplate.EXPECT().GetItemByKey(gomock.Any(), testNoticeKey, "").Return(
				messengerTemplate(templateentity.Variable{Name: "name", DefaultValue: "Guest"}),
				nil,
			)

			uc := usecase.New(serviceTemplate)

			notices, err := uc.Execute(context.Background(), entity.Note{Key: testNoticeKey, Data: tt.data})
			require.NoError(t, err)
			require.Len(t, notices, 1)
			require.NotNil(t, notices[0].Data.Messenger)

			assert.Equal(t, tt.wantContent, notices[0].Data.Messenger.Content)
		})
	}
}

// TestBuildNotice_Execute_NoteDataIsNotModified - значения по умолчанию из шаблона подставляются
// и при отсутствии данных уведомления, а исходные данные уведомления не меняются.
func TestBuildNotice_Execute_NoteDataIsNotModified(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data map[string]string
	}{
		{name: "nil data"},
		{name: "empty data", data: map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			serviceTemplate := mock.NewMocktemplateService(ctrl)
			serviceTemplate.EXPECT().GetItemByKey(gomock.Any(), testNoticeKey, "").Return(
				messengerTemplate(templateentity.Variable{Name: "name", DefaultValue: "Guest"}),
				nil,
			)

			uc := usecase.New(serviceTemplate)

			notices, err := uc.Execute(context.Background(), entity.Note{Key: testNoticeKey, Data: tt.data})
			require.NoError(t, err)
			require.Len(t, notices, 1)
			require.NotNil(t, notices[0].Data.Messenger)

			assert.Equal(t, "Hello, Guest", notices[0].Data.Messenger.Content)
			assert.Empty(t, tt.data)
		})
	}
}

func TestBuildNotice_Execute_DelayTime(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(time.Hour).Truncate(time.Second)

	tests := []struct {
		name      string
		delayTime string
		want      time.Time // нулевое значение - отправка без задержки
	}{
		{name: "seconds", delayTime: "120", want: time.Now().UTC().Add(120 * time.Second)},
		{name: "duration", delayTime: "2h", want: time.Now().UTC().Add(2 * time.Hour)},
		{name: "future rfc3339", delayTime: future.In(time.FixedZone("UTC+3", 3*3600)).Format(time.RFC3339), want: future.UTC()},
		{name: "past rfc3339", delayTime: time.Now().Add(-time.Hour).Format(time.RFC3339)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			serviceTemplate := mock.NewMocktemplateService(ctrl)
			serviceTemplate.EXPECT().GetItemByKey(gomock.Any(), testNoticeKey, "").Return(messengerTemplate(), nil)

			uc := usecase.New(serviceTemplate)

			notices, err := uc.Execute(context.Background(), entity.Note{
				Key:  testNoticeKey,
				Data: map[string]string{mrnotifier.ConfigDelayTime: tt.delayTime},
			})
			require.NoError(t, err)
			require.Len(t, notices, 1)

			if tt.want.IsZero() {
				assert.True(t, notices[0].SendAfter.IsZero())

				return
			}

			// время отправки хранится в UTC
			assert.Equal(t, time.UTC, notices[0].SendAfter.Location())
			assert.WithinDuration(t, tt.want, notices[0].SendAfter, time.Minute)
		})
	}
}

func TestBuildNotice_Execute_IncorrectDelayTime(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	serviceTemplate := mock.NewMocktemplateService(ctrl)
	serviceTemplate.EXPECT().GetItemByKey(gomock.Any(), testNoticeKey, "").Return(messengerTemplate(), nil)

	uc := usecase.New(serviceTemplate)

	notices, err := uc.Execute(context.Background(), entity.Note{
		Key:  testNoticeKey,
		Data: map[string]string{mrnotifier.ConfigDelayTime: "tomorrow"},
	})
	require.ErrorIs(t, err, errors.ErrInternalIncorrectInputData)
	assert.Nil(t, notices)
}

func TestBuildNotice_Execute_TemplateError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	errTemplate := errors.New("template failed")

	serviceTemplate := mock.NewMocktemplateService(ctrl)
	serviceTemplate.EXPECT().GetItemByKey(gomock.Any(), testNoticeKey, "").Return(templatedto.Template{}, errTemplate)

	uc := usecase.New(serviceTemplate)

	notices, err := uc.Execute(context.Background(), entity.Note{Key: testNoticeKey, Data: map[string]string{}})
	require.ErrorIs(t, err, errTemplate)
	assert.Nil(t, notices)
}

func TestBuildNotice_Execute_BuildError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	templ := messengerTemplate()
	templ.Props.Messenger.Content = "" // пустой контент шаблона не собирается

	serviceTemplate := mock.NewMocktemplateService(ctrl)
	serviceTemplate.EXPECT().GetItemByKey(gomock.Any(), testNoticeKey, "").Return(templ, nil)

	uc := usecase.New(serviceTemplate)

	notices, err := uc.Execute(context.Background(), entity.Note{Key: testNoticeKey, Data: map[string]string{}})
	require.Error(t, err)
	assert.Nil(t, notices)
}

func TestBuildNotice_Execute_NoProviders(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	templ := messengerTemplate()
	templ.Props.Messenger.IsDisabled = true

	serviceTemplate := mock.NewMocktemplateService(ctrl)
	serviceTemplate.EXPECT().GetItemByKey(gomock.Any(), testNoticeKey, "").Return(templ, nil)

	uc := usecase.New(serviceTemplate)

	notices, err := uc.Execute(context.Background(), entity.Note{Key: testNoticeKey, Data: map[string]string{}})
	// внутренний сентинел не экспортируется, поэтому причина проверяется по тексту
	require.ErrorContains(t, err, "no providers")
	assert.Nil(t, notices)
}
