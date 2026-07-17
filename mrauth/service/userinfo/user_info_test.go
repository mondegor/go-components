package userinfo_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/service/userinfo"
)

type fakeTx struct{}

func (fakeTx) Do(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
	return job(ctx)
}

type fakeUserFetcher struct {
	user entity.User
}

func (f fakeUserFetcher) FetchOne(context.Context, uuid.UUID) (entity.User, error) {
	return f.user, nil
}

// fake2FAFetcher - имитирует отсутствие записи 2FA (её отсутствие не является ошибкой).
type fake2FAFetcher struct{}

func (fake2FAFetcher) FetchOne(context.Context, uuid.UUID) (entity.Auth2FA, error) {
	return entity.Auth2FA{}, errors.ErrEventStorageNoRecordFound
}

type fakeStatFetcher struct {
	rows []entity.UserActivityStat
}

func (f fakeStatFetcher) Fetch(context.Context, uuid.UUID) ([]entity.UserActivityStat, error) {
	return f.rows, nil
}

type fakeRealmFetcher struct {
	rows []entity.UserRealm
}

func (f fakeRealmFetcher) Fetch(context.Context, uuid.UUID) ([]entity.UserRealm, error) {
	return f.rows, nil
}

func TestUserInfo_Get(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()

	sv := userinfo.New(
		fakeTx{},
		fakeUserFetcher{user: entity.User{ID: userID, Email: "u@example.com"}},
		fake2FAFetcher{},
		fakeStatFetcher{rows: []entity.UserActivityStat{
			// статистика есть только для realm 1
			{RealmID: 1, LastLoginIP: netip.MustParseAddr("203.0.113.7"), LastLoggedAt: base.Add(time.Hour)},
		}},
		fakeRealmFetcher{rows: []entity.UserRealm{
			{RealmID: 1, Kind: "admin", CreatedAt: base, UpdatedAt: base},
			{RealmID: 2, Kind: "standard", CreatedAt: base, UpdatedAt: base},
		}},
		// статистика входа запрашивается только в режиме LocationOrIP
		func(ip netip.Addr, result mrauth.LocationMode) string {
			if result != mrauth.LocationOrIP {
				return "unexpected mode"
			}

			if ip.String() == "203.0.113.7" {
				return "Moscow, RU"
			}

			return ip.String()
		},
	)

	info, err := sv.Get(context.Background(), userID)
	require.NoError(t, err)

	assert.Equal(t, "u@example.com", info.User.Email)
	require.Len(t, info.Realms, 2)

	// realm 1: статистика есть, IP резолвится в место
	assert.Equal(t, uint16(1), info.Realms[0].RealmID)
	assert.Equal(t, "admin", info.Realms[0].Kind)
	assert.Equal(t, "Moscow, RU", info.Realms[0].LastLocation)
	assert.Equal(t, base.Add(time.Hour), info.Realms[0].LastLoggedAt)

	// realm 2: статистики нет - пустое место и нулевое время входа
	assert.Equal(t, uint16(2), info.Realms[1].RealmID)
	assert.Empty(t, info.Realms[1].LastLocation)
	assert.True(t, info.Realms[1].LastLoggedAt.IsZero())
}
