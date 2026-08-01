package httpv1_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/mock"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
)

//go:generate mockgen -source=session.go -destination=mock/session.go -package=mock

// TestSessionGetListRealmSource - realm приходит query-параметром, а не телом запроса:
// отсутствующий параметр означает realm текущей сессии, поэтому в usecase уходит пустая строка.
func TestSessionGetListRealmSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		url       string
		wantRealm string
	}{
		{
			name:      "realm is not specified",
			url:       "/v1/sessions",
			wantRealm: "",
		},
		{
			name:      "realm is specified",
			url:       "/v1/sessions?realm=domain_name/user_group",
			wantRealm: "domain_name/user_group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			parser := mock.NewMockRequestParser(ctrl)
			sender := mock.NewMockResponseSender(ctrl)
			useCase := mock.NewMocksessionUseCase(ctrl)

			parser.EXPECT().RawParamString(gomock.Any(), "realm").DoAndReturn(rawParamString)
			parser.EXPECT().ValidateStruct(gomock.Any(), gomock.Any()).Return(nil)
			parser.EXPECT().UserID(gomock.Any()).Return(uuid.New())
			parser.EXPECT().Location(gomock.Any()).Return(time.UTC)

			useCase.EXPECT().
				GetList(gomock.Any(), gomock.Any(), gomock.Any(), tt.wantRealm).
				Return([]dto.UserSession{}, nil)
			sender.EXPECT().Send(gomock.Any(), http.StatusOK, gomock.Any()).Return(nil)

			controller := httpv1.NewSession(parser, sender, useCase)

			require.NoError(
				t,
				controller.GetList(
					httptest.NewRecorder(),
					httptest.NewRequest(http.MethodGet, tt.url, http.NoBody),
				),
			)
		})
	}
}

// TestSessionGetListPassesEmptyRealmToValidator - "?realm=" это не то же самое, что отсутствующий
// параметр: пустое значение должно доехать до валидатора (там его отсекает minLength), иначе
// оно молча подменилось бы realm'ом текущей сессии.
func TestSessionGetListPassesEmptyRealmToValidator(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	parser := mock.NewMockRequestParser(ctrl)
	useCase := mock.NewMocksessionUseCase(ctrl)
	errValidate := errors.New("realm is invalid")

	parser.EXPECT().RawParamString(gomock.Any(), "realm").DoAndReturn(rawParamString)
	parser.EXPECT().
		ValidateStruct(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, structPointer any) error {
			req, ok := structPointer.(*model.UserSessionsRequest)
			require.True(t, ok)
			require.NotNil(t, req.Realm, "присланный пустой realm не должен превращаться в nil")
			assert.Empty(t, *req.Realm)

			return errValidate
		})

	// usecase не вызывается: запрос не прошёл валидацию
	controller := httpv1.NewSession(parser, nil, useCase)

	require.ErrorIs(
		t,
		controller.GetList(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/v1/sessions?realm=", http.NoBody),
		),
		errValidate,
	)
}

// TestSessionGetListRoute - список сессий только читает данные, поэтому отдаётся по GET;
// тест держит соответствие со спекой, т.к. возврат к POST не сломал бы компиляцию.
func TestSessionGetListRoute(t *testing.T) {
	t.Parallel()

	// зависимости не нужны: Handlers() лишь собирает список, обработчики не вызываются
	controller := httpv1.NewSession(nil, nil, nil)

	handlerByURL := make(map[string]string)

	for _, h := range controller.Handlers() {
		if h.Method == http.MethodGet {
			handlerByURL[h.URL] = handlerName(h.Func)
		}
	}

	getList, ok := handlerByURL["/v1/sessions"]
	require.True(t, ok, "список открытых сессий должен отдаваться по GET /v1/sessions")
	assert.Equal(t, "GetList", getList)
}

// rawParamString - подменяет разбор query-параметра реализацией go-webcore:
// nil, если ключа нет, и указатель на значение (в т.ч. пустое), если ключ присутствует.
func rawParamString(r *http.Request, key string) *string {
	if !r.URL.Query().Has(key) {
		return nil
	}

	value := r.URL.Query().Get(key)

	return &value
}
