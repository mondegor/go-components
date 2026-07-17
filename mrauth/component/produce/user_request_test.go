package produce_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/component/produce"
	"github.com/mondegor/go-components/mrauth/dto"
)

type fakeUserLogProducer struct {
	captured []dto.UserActivityLogMessage
}

func (f *fakeUserLogProducer) PushMessage(_ context.Context, message dto.UserActivityLogMessage) error {
	f.captured = append(f.captured, message)

	return nil
}

type fakeParserUser struct {
	userID    uuid.UUID
	group     string
	sessionID string
}

func (f *fakeParserUser) UserID(*http.Request) uuid.UUID { return f.userID }

func (f *fakeParserUser) UserAndGroup(*http.Request) (uuid.UUID, string) {
	return f.userID, f.group
}

func (f *fakeParserUser) SessionID(*http.Request) string { return f.sessionID }

type fakeParserClientIP struct{}

func (fakeParserClientIP) RealIP(*http.Request) netip.Addr { return netip.Addr{} }

func (fakeParserClientIP) DetailedIP(*http.Request) mrtype.DetailedIP { return mrtype.DetailedIP{} }

type fakeRealmRegistry struct {
	ids map[string]uint16
}

func (f fakeRealmRegistry) IDByName(name string) (uint16, bool) {
	id, ok := f.ids[name]

	return id, ok
}

func (fakeRealmRegistry) NameByID(uint16) (string, bool) { return "", false }

// countingLogger - считает предупреждения; остальные методы остаются no-op от вложенного логгера.
type countingLogger struct {
	mrlog.Logger

	errors int
}

func (l *countingLogger) Error(context.Context, string, ...any) {
	l.errors++
}

func TestUserRequest_Emit_ResolvesRealm(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	producer := &fakeUserLogProducer{}

	rs := produce.NewUserRequest(
		producer,
		mrlog.NopLogger(),
		fakeParserClientIP{},
		// group формата "{realm}/{kind}", realm может содержать '/'
		&fakeParserUser{userID: userID, group: "site/admin/manager", sessionID: "0"},
		fakeRealmRegistry{ids: map[string]uint16{"site/admin": 7}},
	)

	rs.Emit(httptest.NewRequest(http.MethodGet, "/x", http.NoBody), nil, 0, nil, 0, 0, http.StatusOK)

	require.Len(t, producer.captured, 1)
	assert.Equal(t, userID, producer.captured[0].UserID)
	assert.Equal(t, uint16(7), producer.captured[0].RealmID)
}

// TestUserRequest_Emit_UnknownRealmSentinel - промах реестра realm'ов не дропает сообщение
// (иначе замёрз бы keep-alive сессий), а помечает его сентинелом RealmID = 0.
func TestUserRequest_Emit_UnknownRealmSentinel(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	producer := &fakeUserLogProducer{}

	rs := produce.NewUserRequest(
		producer,
		mrlog.NopLogger(),
		fakeParserClientIP{},
		&fakeParserUser{userID: userID, group: "unknown/kind"},
		fakeRealmRegistry{ids: map[string]uint16{}},
	)

	rs.Emit(httptest.NewRequest(http.MethodGet, "/x", http.NoBody), nil, 0, nil, 0, 0, http.StatusOK)

	require.Len(t, producer.captured, 1)
	assert.Equal(t, userID, producer.captured[0].UserID)
	assert.Equal(t, uint16(0), producer.captured[0].RealmID)
}

// TestUserRequest_Emit_ErrorsOnceOnUnknownRealm - Emit вызывается на каждый http-ответ,
// поэтому промах реестра не должен заливать логи предупреждениями: в пределах периода
// троттлинга пишется одно сообщение (протухание периода проверяется в internal-тесте),
// при этом сами сообщения активности продолжают уходить с сентинелом RealmID = 0.
func TestUserRequest_Emit_ErrorsOnceOnUnknownRealm(t *testing.T) {
	t.Parallel()

	producer := &fakeUserLogProducer{}
	logger := &countingLogger{Logger: mrlog.NopLogger()}

	rs := produce.NewUserRequest(
		producer,
		logger,
		fakeParserClientIP{},
		&fakeParserUser{userID: uuid.New(), group: "unknown/kind"},
		fakeRealmRegistry{ids: map[string]uint16{}},
	)

	for range 3 {
		rs.Emit(httptest.NewRequest(http.MethodGet, "/x", http.NoBody), nil, 0, nil, 0, 0, http.StatusOK)
	}

	require.Len(t, producer.captured, 3)
	assert.Equal(t, 1, logger.errors)
}

func TestUserRequest_Emit_OnlyRealm(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	producer := &fakeUserLogProducer{}

	rs := produce.NewUserRequest(
		producer,
		mrlog.NopLogger(),
		fakeParserClientIP{},
		&fakeParserUser{userID: userID, group: "realm"},
		fakeRealmRegistry{ids: map[string]uint16{"realm": 7}},
	)

	rs.Emit(httptest.NewRequest(http.MethodGet, "/x", http.NoBody), nil, 0, nil, 0, 0, http.StatusOK)

	require.Len(t, producer.captured, 1)
	assert.Equal(t, userID, producer.captured[0].UserID)
	assert.Equal(t, uint16(7), producer.captured[0].RealmID)
}

func TestUserRequest_Emit_SkipsAnonymous(t *testing.T) {
	t.Parallel()

	producer := &fakeUserLogProducer{}

	rs := produce.NewUserRequest(
		producer,
		mrlog.NopLogger(),
		fakeParserClientIP{},
		&fakeParserUser{userID: uuid.Nil},
		fakeRealmRegistry{},
	)

	rs.Emit(httptest.NewRequest(http.MethodGet, "/x", http.NoBody), nil, 0, nil, 0, 0, http.StatusOK)

	assert.Empty(t, producer.captured)
}
