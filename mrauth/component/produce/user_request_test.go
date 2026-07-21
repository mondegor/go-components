package produce_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/component/produce"
	"github.com/mondegor/go-components/mrauth/component/produce/mock"
	"github.com/mondegor/go-components/mrauth/dto"
)

//go:generate mockgen -source=user_request.go -destination=mock/user_request.go -package=mock
//go:generate mockgen -source=secure_operation.go -destination=mock/secure_operation.go -package=mock
//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth RealmRegistry
//go:generate mockgen -destination=mock/mrserver.go -package=mock github.com/mondegor/go-webcore/mrserver/request ParserUser,ParserClientIP

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

	ctrl       *gomock.Controller
	producer   *mock.MockuserLogProducer
	parserIP   *mock.MockParserClientIP
	parserUser *mock.MockParserUser
	registry   *mock.MockRealmRegistry
	captured   []dto.UserActivityLogMessage
}

func TestUserRequestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(UserRequestSuite))
}

func (s *UserRequestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.producer = mock.NewMockuserLogProducer(s.ctrl)
	s.parserIP = mock.NewMockParserClientIP(s.ctrl)
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

	s.parserIP.EXPECT().RealIP(gomock.Any()).Return(netip.Addr{}).AnyTimes()
	s.parserIP.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{}).AnyTimes()
	// продюсер запросов поясом не пользуется
	s.parserUser.EXPECT().Location(gomock.Any()).Return(time.UTC).AnyTimes()
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
	rs := produce.NewUserRequest(s.producer, logger, s.parserIP, s.parserUser, s.registry)
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

	rs := produce.NewUserRequest(s.producer, logger, s.parserIP, s.parserUser, s.registry)

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
