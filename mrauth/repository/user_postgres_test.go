package repository_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrtype"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const usersTableName = "sample_schema.users"

type UserPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.UserPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestUserPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(UserPostgresTestSuite))
}

func (ts *UserPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))
	ts.repo = repository.NewUserPostgres(ts.pgt.ConnManager(), usersTableName)
}

func (ts *UserPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// fixtureUpdatedAt - updated_at пользователя из фикстуры (заведомо в прошлом),
// чтобы наблюдать его сдвиг после обновления (updated_at = NOW()).
func (ts *UserPostgresTestSuite) fixtureUpdatedAt() time.Time {
	return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
}

// Test_UpdateEmailWhenDuplicate - адрес, занятый другим пользователем, не присваивается: уникальность
// держит индекс, и repo отдаёт ErrInternalStorageDuplicateKeyViolation. На этом сентинеле
// обработчик смены емаила строит ответ EmailAlreadyExists, поэтому контракт пиннится здесь.
func (ts *UserPostgresTestSuite) Test_UpdateEmailWhenDuplicate() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/User/UpdateEmailWhenDuplicate")

	userID := uuid.MustParse(fixtureUserB)

	// адрес занят пользователем A
	err := ts.repo.UpdateEmail(ts.ctx, userID, "user-a@localhost")
	ts.Require().ErrorIs(err, errors.ErrInternalStorageDuplicateKeyViolation)

	ts.Require().NoError(ts.repo.UpdateEmail(ts.ctx, userID, "free@localhost"))
}

// Test_UpdateSettings - обновляет язык и часовой пояс одним запросом и сдвигает updated_at.
func (ts *UserPostgresTestSuite) Test_UpdateSettings() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/User/UpdateSettings")

	userID := uuid.MustParse(fixtureUserA)

	err := ts.repo.UpdateSettings(ts.ctx, entity.UserSettings{
		UserID:   userID,
		LangCode: "en-US",
		TimeZone: "UTC",
	})
	ts.Require().NoError(err)

	row, err := ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal("en-US", row.LangCode)
	ts.Equal("UTC", row.TimeZone)
	ts.True(row.UpdatedAt.After(ts.fixtureUpdatedAt()), "updated_at должен быть обновлён на NOW()")
}

// Test_UpdateSettingsWhenUserNotExists - обновление несуществующего пользователя не затрагивает строк
// и возвращает ErrEventStorageNoRecordFound (0 строк в ExecRow), ничего не создавая.
func (ts *UserPostgresTestSuite) Test_UpdateSettingsWhenUserNotExists() {
	err := ts.repo.UpdateSettings(ts.ctx, entity.UserSettings{
		UserID:   uuid.MustParse(fixtureUserC),
		LangCode: "en-US",
		TimeZone: "UTC",
	})
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_UpdateSettingsWhenUserDeleted - мягко удалённый пользователь не обновляется
// (условие deleted_at IS NULL): возвращается ErrEventStorageNoRecordFound.
func (ts *UserPostgresTestSuite) Test_UpdateSettingsWhenUserDeleted() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/User/UpdateSettingsWhenUserDeleted")

	userID := uuid.MustParse(fixtureUserA)

	err := ts.repo.UpdateSettings(ts.ctx, entity.UserSettings{
		UserID:   userID,
		LangCode: "en-US",
		TimeZone: "UTC",
	})
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_Insert - пользователь с телефоном и прокси-адресом и пользователь без них сохраняются
// без искажений (отсутствующие значения - NULL); email живого пользователя повторно не занимается.
func (ts *UserPostgresTestSuite) Test_Insert() {
	full := entity.ExtendedUser{
		User: entity.User{
			ID:       uuid.MustParse(fixtureUserA),
			Email:    "user-a@localhost",
			Phone:    9876543210,
			LangCode: "ru-RU",
			TimeZone: "Europe/Moscow",
			Status:   userstatus.Enabled,
		},
		RegisteredIP: mrtype.NewDetailedIP(netip.MustParseAddr("203.0.113.7"), netip.MustParseAddr("10.0.0.1")),
	}

	bare := entity.ExtendedUser{
		User: entity.User{
			ID:       uuid.MustParse(fixtureUserB),
			Email:    "user-b@localhost",
			LangCode: "en-US",
			TimeZone: "UTC",
			Status:   userstatus.Draft,
		},
		RegisteredIP: mrtype.NewIP(netip.MustParseAddr("2001:db8::1")),
	}

	ts.Require().NoError(ts.repo.Insert(ts.ctx, full))
	ts.Require().NoError(ts.repo.Insert(ts.ctx, bare))

	for _, expected := range []entity.ExtendedUser{full, bare} {
		got, err := ts.repo.FetchOne(ts.ctx, expected.ID)
		ts.Require().NoError(err)

		ts.Equal(expected.Email, got.Email)
		ts.Equal(expected.Phone, got.Phone)
		ts.Equal(expected.LangCode, got.LangCode)
		ts.Equal(expected.TimeZone, got.TimeZone)
		ts.Equal(expected.Status, got.Status)

		var (
			realIP, proxyIP netip.Addr
			isPhoneNull     bool
		)

		err = ts.pgt.ConnManager().Conn(ts.ctx).QueryRow(
			ts.ctx,
			`SELECT registered_ip, registered_proxy_ip, user_phone IS NULL FROM `+usersTableName+` WHERE user_id = $1;`,
			expected.ID,
		).Scan(&realIP, &proxyIP, &isPhoneNull)
		ts.Require().NoError(err)
		ts.Equal(expected.RegisteredIP.Real, realIP)
		ts.Equal(expected.RegisteredIP.Proxy, proxyIP)
		ts.Equal(expected.Phone == 0, isPhoneNull)
	}

	duplicate := bare
	duplicate.ID = uuid.MustParse(fixtureUserC)
	duplicate.Email = full.Email

	err := ts.repo.Insert(ts.ctx, duplicate)
	ts.Require().ErrorIs(err, errors.ErrInternalStorageDuplicateKeyViolation)
}

// Test_FetchOneByLogin - пользователь находится как по email, так и по телефону.
func (ts *UserPostgresTestSuite) Test_FetchOneByLogin() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/User/FetchOneByLogin")

	userID := uuid.MustParse(fixtureUserA)

	row, err := ts.repo.FetchOneByLogin(ts.ctx, contactaddress.NewEmail("user-a@localhost"))
	ts.Require().NoError(err)
	ts.Equal(userID, row.ID)
	ts.Equal(uint64(9876543210), row.Phone)

	row, err = ts.repo.FetchOneByLogin(ts.ctx, contactaddress.NewDigitPhone(9876543210))
	ts.Require().NoError(err)
	ts.Equal(userID, row.ID)
	ts.Equal("user-a@localhost", row.Email)
}

// Test_FetchOneByLoginWhenUserDeleted - мягко удалённый пользователь по логину не находится.
func (ts *UserPostgresTestSuite) Test_FetchOneByLoginWhenUserDeleted() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/User/FetchOneByLoginWhenUserDeleted")

	_, err := ts.repo.FetchOneByLogin(ts.ctx, contactaddress.NewEmail("user-a@localhost"))
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOneByLogin(ts.ctx, contactaddress.NewDigitPhone(9876543210))
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_FetchOneByLoginWhenNotFound - неизвестный логин возвращает ErrEventStorageNoRecordFound.
func (ts *UserPostgresTestSuite) Test_FetchOneByLoginWhenNotFound() {
	_, err := ts.repo.FetchOneByLogin(ts.ctx, contactaddress.NewEmail("unknown@localhost"))
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOneByLogin(ts.ctx, contactaddress.NewDigitPhone(9000000001))
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_FetchOneByLoginWhenAddressUnsupported - адрес без типа - внутренняя ошибка, а не «не найден»:
// такой логин до хранилища дойти не должен.
func (ts *UserPostgresTestSuite) Test_FetchOneByLoginWhenAddressUnsupported() {
	_, err := ts.repo.FetchOneByLogin(ts.ctx, contactaddress.ContactAddress{})
	ts.Require().Error(err)
	ts.NotErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_UpdatePhone - меняет телефон пользователя и сдвигает updated_at.
func (ts *UserPostgresTestSuite) Test_UpdatePhone() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/User/UpdatePhone")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.UpdatePhone(ts.ctx, userID, 9000000001))

	row, err := ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal(uint64(9000000001), row.Phone)
	ts.True(row.UpdatedAt.After(ts.fixtureUpdatedAt()), "updated_at должен быть обновлён на NOW()")
}

// Test_UpdatePhoneWhenDuplicate - телефон, занятый другим пользователем, не присваивается:
// repo отдаёт ErrInternalStorageDuplicateKeyViolation.
func (ts *UserPostgresTestSuite) Test_UpdatePhoneWhenDuplicate() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/User/UpdatePhoneWhenDuplicate")

	// телефон занят пользователем B
	err := ts.repo.UpdatePhone(ts.ctx, uuid.MustParse(fixtureUserA), 9000000002)
	ts.Require().ErrorIs(err, errors.ErrInternalStorageDuplicateKeyViolation)
}

// Test_UpdatePhoneWhenUserNotExists - обновление несуществующего пользователя возвращает
// ErrEventStorageNoRecordFound.
func (ts *UserPostgresTestSuite) Test_UpdatePhoneWhenUserNotExists() {
	err := ts.repo.UpdatePhone(ts.ctx, uuid.MustParse(fixtureUserA), 9000000001)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_UpdatePhoneWhenUserDeleted - мягко удалённый пользователь не обновляется:
// возвращается ErrEventStorageNoRecordFound.
func (ts *UserPostgresTestSuite) Test_UpdatePhoneWhenUserDeleted() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/User/UpdatePhoneWhenUserDeleted")

	err := ts.repo.UpdatePhone(ts.ctx, uuid.MustParse(fixtureUserA), 9000000001)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_FetchOne - пользователь читается по идентификатору без искажений.
func (ts *UserPostgresTestSuite) Test_FetchOne() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/User/FetchOne")

	got, err := ts.repo.FetchOne(ts.ctx, uuid.MustParse(fixtureUserA))
	ts.Require().NoError(err)

	ts.Equal(uuid.MustParse(fixtureUserA), got.ID)
	ts.Equal("user-a@localhost", got.Email)
	ts.Equal(uint64(9876543210), got.Phone)
	ts.Equal("ru-RU", got.LangCode)
	ts.Equal("Europe/Moscow", got.TimeZone)
	ts.Equal(userstatus.Enabled, got.Status)
	ts.True(time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC).Equal(got.CreatedAt))
	ts.True(ts.fixtureUpdatedAt().Equal(got.UpdatedAt))
}

// Test_FetchOneWhenUserDeleted - мягко удалённый пользователь не находится: ErrEventStorageNoRecordFound.
func (ts *UserPostgresTestSuite) Test_FetchOneWhenUserDeleted() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/User/FetchOneWhenUserDeleted")

	_, err := ts.repo.FetchOne(ts.ctx, uuid.MustParse(fixtureUserA))
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}
