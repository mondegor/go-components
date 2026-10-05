package collect_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/infra/adapter/collect"
	"github.com/mondegor/go-components/mrauth/infra/adapter/collect/mock"
)

//go:generate mockgen -source=user_request.go -destination=mock/user_request.go -package=mock
//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth RealmRegistry
//go:generate mockgen -destination=mock/mrserver.go -package=mock github.com/mondegor/go-webcore/mrserver/request ParserUser,ParserClient

const testUserAgent = "Mozilla/5.0 (X11)"

// countingLogger - считает вызовы Error; остальные методы остаются no-op от вложенного логгера.
//
// Это не мок коллаборатора, а зонд для проверки троттлинга логирования: мок mrlog.Logger
// потребовал бы AnyTimes()-заглушек на весь интерфейс логгера ради одного счётчика,
// поэтому правило "моки только через mockgen" здесь сознательно не применяется.
type countingLogger struct {
	mrlog.Logger

	errors int
}

func (l *countingLogger) Error(context.Context, string, ...any) {
	l.errors++
}

type UserRequestSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	producer     *mock.MockuserLogProducer
	parserClient *mock.MockParserClient
	parserUser   *mock.MockParserUser
	registry     *mock.MockRealmRegistry
	captured     []dto.UserActivityLogMessage
}

func TestUserRequestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(UserRequestSuite))
}

func (s *UserRequestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.producer = mock.NewMockuserLogProducer(s.ctrl)
	s.parserClient = mock.NewMockParserClient(s.ctrl)
	s.parserUser = mock.NewMockParserUser(s.ctrl)
	s.registry = mock.NewMockRealmRegistry(s.ctrl)
	s.captured = nil

	s.producer.EXPECT().
		PushMessage(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, message dto.UserActivityLogMessage) error {
			s.captured = append(s.captured, message)

			return nil
		}).
		AnyTimes()

	s.parserClient.EXPECT().RealIP(gomock.Any()).Return(netip.Addr{}).AnyTimes()
	s.parserClient.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{}).AnyTimes()
	s.parserClient.EXPECT().UserAgent(gomock.Any()).Return(testUserAgent).AnyTimes()
}

// expectUser - фиксирует пользователя и его группу формата "{realm}/{kind}".
func (s *UserRequestSuite) expectUser(userID uuid.UUID, group, sessionID string) {
	s.parserUser.EXPECT().UserID(gomock.Any()).Return(userID).AnyTimes()
	s.parserUser.EXPECT().UserAndGroup(gomock.Any()).Return(userID, group).AnyTimes()
	s.parserUser.EXPECT().SessionID(gomock.Any()).Return(sessionID).AnyTimes()
}

// expectRealms - настраивает реестр realm'ов; отсутствующее имя даёт промах.
func (s *UserRequestSuite) expectRealms(ids map[string]uint16) {
	s.registry.EXPECT().
		IDByName(gomock.Any()).
		DoAndReturn(func(name string) (uint16, bool) {
			id, ok := ids[name]

			return id, ok
		}).
		AnyTimes()
}

func (s *UserRequestSuite) emit(logger mrlog.Logger) {
	rs := collect.NewUserRequest(s.producer, logger, s.parserClient, s.parserUser, s.registry)
	rs.Emit(httptest.NewRequest(http.MethodGet, "/x", http.NoBody), nil, 0, nil, 0, 0, http.StatusOK)
}

func (s *UserRequestSuite) TestEmitResolvesRealm() {
	userID := uuid.New()

	// group формата "{realm}/{kind}", realm может содержать '/'
	s.expectUser(userID, "site/admin/manager", "0")
	s.expectRealms(map[string]uint16{"site/admin": 7})

	s.emit(mrlog.NopLogger())

	s.Require().Len(s.captured, 1)
	s.Equal(userID, s.captured[0].UserID)
	s.Equal(uint16(7), s.captured[0].RealmID)
}

// TestEmitUnknownRealmSentinel - промах реестра realm'ов не дропает сообщение
// (иначе замёрз бы keep-alive сессий), а помечает его сентинелом RealmID = 0.
func (s *UserRequestSuite) TestEmitUnknownRealmSentinel() {
	userID := uuid.New()

	s.expectUser(userID, "unknown/kind", "")
	s.expectRealms(map[string]uint16{})

	s.emit(mrlog.NopLogger())

	s.Require().Len(s.captured, 1)
	s.Equal(userID, s.captured[0].UserID)
	s.Equal(uint16(0), s.captured[0].RealmID)
}

// TestEmitErrorsOnceOnUnknownRealm - Emit вызывается на каждый http-ответ,
// поэтому промах реестра не должен заливать логи предупреждениями: в пределах периода
// троттлинга пишется одно сообщение (протухание периода проверяется в internal-тесте),
// при этом сами сообщения активности продолжают уходить с сентинелом RealmID = 0.
func (s *UserRequestSuite) TestEmitErrorsOnceOnUnknownRealm() {
	logger := &countingLogger{Logger: mrlog.NopLogger()}

	s.expectUser(uuid.New(), "unknown/kind", "")
	s.expectRealms(map[string]uint16{})

	rs := collect.NewUserRequest(s.producer, logger, s.parserClient, s.parserUser, s.registry)

	for range 3 {
		rs.Emit(httptest.NewRequest(http.MethodGet, "/x", http.NoBody), nil, 0, nil, 0, 0, http.StatusOK)
	}

	s.Require().Len(s.captured, 3)
	s.Equal(1, logger.errors)
}

func (s *UserRequestSuite) TestEmitOnlyRealm() {
	userID := uuid.New()

	s.expectUser(userID, "realm", "")
	s.expectRealms(map[string]uint16{"realm": 7})

	s.emit(mrlog.NopLogger())

	s.Require().Len(s.captured, 1)
	s.Equal(userID, s.captured[0].UserID)
	s.Equal(uint16(7), s.captured[0].RealmID)
}

func (s *UserRequestSuite) TestEmitSkipsAnonymous() {
	s.expectUser(uuid.Nil, "", "")
	s.expectRealms(map[string]uint16{})

	s.emit(mrlog.NopLogger())

	s.Empty(s.captured)
}

// TestEmitTakesUserAgentFromParser - user agent в сообщение активности берётся из парсера
// запроса (он приводит заголовок к безопасному виду), а не из сырого заголовка.
func (s *UserRequestSuite) TestEmitTakesUserAgentFromParser() {
	s.expectUser(uuid.New(), "site/kind", "")
	s.expectRealms(map[string]uint16{"site": 1})

	r := httptest.NewRequest(http.MethodGet, "/x", http.NoBody)
	r.Header.Set("User-Agent", "raw-agent")

	collect.NewUserRequest(s.producer, mrlog.NopLogger(), s.parserClient, s.parserUser, s.registry).
		Emit(r, nil, 0, nil, 0, 0, http.StatusOK)

	s.Require().Len(s.captured, 1)
	s.Equal(testUserAgent, s.captured[0].UserAgent)
}

// TestEmitSanitizesRequestPath - путь запроса в сообщении активности приведён к виду, пригодному
// для колонки журнала: без невалидного UTF-8 и NUL и не длиннее её ширины.
func (s *UserRequestSuite) TestEmitSanitizesRequestPath() {
	s.expectUser(uuid.New(), "site/kind", "")
	s.expectRealms(map[string]uint16{"site": 1})

	tests := []struct {
		name   string
		target string
		want   string
	}{
		{name: "plain", target: "/v1/user/settings", want: "/v1/user/settings"},
		{name: "invalid utf8 and nul", target: "/a%FFb%00c", want: "/abc"},
		{name: "too long", target: "/" + strings.Repeat("a", 300), want: "/" + strings.Repeat("a", 255)},
	}

	for _, tt := range tests {
		s.captured = nil

		r := httptest.NewRequest(http.MethodGet, tt.target, http.NoBody)

		collect.NewUserRequest(s.producer, mrlog.NopLogger(), s.parserClient, s.parserUser, s.registry).
			Emit(r, nil, 0, nil, 0, 0, http.StatusOK)

		s.Require().Len(s.captured, 1, tt.name)
		s.Equal(tt.want, s.captured[0].RequestPath, tt.name)
	}
}
