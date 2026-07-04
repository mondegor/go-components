package authuser_test

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/service/authuser"
	"github.com/mondegor/go-components/mrauth/service/realm"
)

type (
	fakeTxManager struct{}

	fakeUserStorage struct {
		byLogin     entity.User
		byLoginErr  error
		fetchOne    entity.User
		fetchOneErr error
		inserted    []entity.ExtendedUser
	}

	fakeUserRealmStorage struct {
		fetchOne    entity.UserRealm
		fetchOneErr error
		insertErr   error
		inserted    []entity.UserRealm
	}

	fakeNotifier struct {
		events []string
	}
)

func (fakeTxManager) Do(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
	return job(ctx)
}

func (f *fakeUserStorage) FetchOne(context.Context, uuid.UUID) (entity.User, error) {
	return f.fetchOne, f.fetchOneErr
}

func (f *fakeUserStorage) FetchOneByLogin(context.Context, contactaddress.ContactAddress) (entity.User, error) {
	return f.byLogin, f.byLoginErr
}

func (f *fakeUserStorage) Insert(_ context.Context, row entity.ExtendedUser) error {
	f.inserted = append(f.inserted, row)

	return nil
}

func (f *fakeUserRealmStorage) FetchOne(context.Context, uuid.UUID, uint16) (entity.UserRealm, error) {
	return f.fetchOne, f.fetchOneErr
}

func (f *fakeUserRealmStorage) Insert(_ context.Context, row entity.UserRealm) error {
	if f.insertErr != nil {
		return f.insertErr
	}

	f.inserted = append(f.inserted, row)

	return nil
}

func (f *fakeNotifier) Send(_ context.Context, key string, _ map[string]any) error {
	f.events = append(f.events, key)

	return nil
}

func newCreateIn() dto.CreateUserOperation {
	return dto.CreateUserOperation{
		Realm:    "site/admin",
		UserKind: "admin",
		LangCode: "en",
		Email:    "user@example.com",
	}
}

func newService(storageUser *fakeUserStorage, storageUserRealm *fakeUserRealmStorage, notifier *fakeNotifier) *authuser.Service {
	realmRegistry := realm.New([]realm.Realm{{ID: 1, Name: "site/admin"}})

	return authuser.New(fakeTxManager{}, storageUser, storageUserRealm, realmRegistry, notifier, mrlog.NopLogger())
}

// новый email (userID=Nil, пользователь не найден) -> создаётся пользователь и привязка к realm,
// возвращается сгенерированный id, отправляются оба уведомления о регистрации.
func TestResolveUser_NewEmail(t *testing.T) {
	t.Parallel()

	storageUser := &fakeUserStorage{byLoginErr: errors.ErrEventStorageNoRecordFound}
	storageUserRealm := &fakeUserRealmStorage{}
	notifier := &fakeNotifier{}

	in := newCreateIn()
	in.RegisteredIP = "203.0.113.7"

	userID, err := newService(storageUser, storageUserRealm, notifier).ResolveUser(context.Background(), uuid.Nil, in)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, userID)

	require.Len(t, storageUser.inserted, 1)
	assert.Equal(t, userID, storageUser.inserted[0].ID)
	assert.Equal(t, "203.0.113.7", storageUser.inserted[0].RegisteredIP, "IP регистрации фиксируется у нового пользователя")
	require.Len(t, storageUserRealm.inserted, 1)
	assert.Equal(t, []string{"user.registration.success.site/admin", "user.was.registered"}, notifier.events)
}

// чистый ретрай того же токена (userID=Nil): пользователь и привязка к realm уже зафиксированы
// предыдущей попыткой -> повторная привязка даёт дубль (уже в realm), ничего не создаётся,
// уведомления не шлются, возвращается существующий id.
func TestResolveUser_RetryExistingUser(t *testing.T) {
	t.Parallel()

	existingID := uuid.New()
	storageUser := &fakeUserStorage{byLogin: entity.User{ID: existingID, Email: "user@example.com"}}
	storageUserRealm := &fakeUserRealmStorage{insertErr: errors.ErrEventRecordAlreadyExists}
	notifier := &fakeNotifier{}

	userID, err := newService(storageUser, storageUserRealm, notifier).ResolveUser(context.Background(), uuid.Nil, newCreateIn())
	require.NoError(t, err)
	assert.Equal(t, existingID, userID)

	assert.Empty(t, storageUser.inserted, "existing user must not be re-inserted")
	assert.Empty(t, storageUserRealm.inserted, "realm binding already committed in the original transaction")
	assert.Empty(t, notifier.events, "registration notifications already sent on the original attempt")
}

// пользователь с этим email уже есть (userID=Nil), но привязки к нужному realm нет (параллельный
// кросс-realm signup или внешнее создание email в зазоре) -> привязка достраивается идемпотентно,
// шлётся уведомление о регистрации в realm (юзеру), пользователь не пересоздаётся,
// возвращается существующий id.
func TestResolveUser_ExistingUserMissingRealmBinding(t *testing.T) {
	t.Parallel()

	existingID := uuid.New()
	storageUser := &fakeUserStorage{byLogin: entity.User{ID: existingID, Email: "user@example.com"}}
	storageUserRealm := &fakeUserRealmStorage{}
	notifier := &fakeNotifier{}

	userID, err := newService(storageUser, storageUserRealm, notifier).ResolveUser(context.Background(), uuid.Nil, newCreateIn())
	require.NoError(t, err)
	assert.Equal(t, existingID, userID)

	assert.Empty(t, storageUser.inserted, "existing user must not be re-inserted")
	require.Len(t, storageUserRealm.inserted, 1, "missing realm binding must be created")
	assert.Equal(t, existingID, storageUserRealm.inserted[0].UserID)
	assert.Equal(t, []string{"user.registration.success.site/admin"}, notifier.events)
}

// известный пользователь (userID!=Nil, привязка к новому realm): поиск по email не выполняется,
// создаётся только привязка к realm, шлётся уведомление о регистрации в realm (юзеру),
// возвращается тот же id.
func TestResolveUser_KnownUserBindsToRealm(t *testing.T) {
	t.Parallel()

	knownID := uuid.New()
	storageUser := &fakeUserStorage{byLoginErr: stderrors.New("FetchOneByLogin must not be called")}
	storageUserRealm := &fakeUserRealmStorage{}
	notifier := &fakeNotifier{}

	userID, err := newService(storageUser, storageUserRealm, notifier).ResolveUser(context.Background(), knownID, newCreateIn())
	require.NoError(t, err)
	assert.Equal(t, knownID, userID)

	assert.Empty(t, storageUser.inserted, "known user must not be inserted")
	require.Len(t, storageUserRealm.inserted, 1)
	assert.Equal(t, knownID, storageUserRealm.inserted[0].UserID)
	assert.Equal(t, []string{"user.registration.success.site/admin"}, notifier.events)
}

// известный пользователь (userID!=Nil), уже привязанный к realm: повторная привязка даёт дубль,
// трактуется как успех - ничего не создаётся, уведомления не шлются, возвращается тот же id.
func TestResolveUser_KnownUserAlreadyInRealm(t *testing.T) {
	t.Parallel()

	knownID := uuid.New()
	storageUser := &fakeUserStorage{}
	storageUserRealm := &fakeUserRealmStorage{insertErr: errors.ErrEventRecordAlreadyExists}
	notifier := &fakeNotifier{}

	userID, err := newService(storageUser, storageUserRealm, notifier).ResolveUser(context.Background(), knownID, newCreateIn())
	require.NoError(t, err)
	assert.Equal(t, knownID, userID)

	assert.Empty(t, storageUserRealm.inserted)
	assert.Empty(t, notifier.events)
}

// жёсткая ошибка поиска по email (не "не найдено") пробрасывается, пользователь не создаётся.
func TestResolveUser_LookupError(t *testing.T) {
	t.Parallel()

	storageUser := &fakeUserStorage{byLoginErr: stderrors.New("db is down")}
	storageUserRealm := &fakeUserRealmStorage{}
	notifier := &fakeNotifier{}

	_, err := newService(storageUser, storageUserRealm, notifier).ResolveUser(context.Background(), uuid.Nil, newCreateIn())
	require.Error(t, err)

	assert.Empty(t, storageUser.inserted)
	assert.Empty(t, storageUserRealm.inserted)
}

// PrepareAuthorization сам НЕ шлёт login-alert: он возвращает scopes и отложенный callback,
// а уведомление user.authorization.success.<realm> уходит только при вызове callback'а.
func TestPrepareAuthorization_DefersRealmSpecificNotice(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	storageUser := &fakeUserStorage{fetchOne: entity.User{ID: userID, Email: "user@example.com", LangCode: "en"}}
	storageUserRealm := &fakeUserRealmStorage{fetchOne: entity.UserRealm{UserID: userID, RealmID: 1, Kind: "admin"}}
	notifier := &fakeNotifier{}

	scopes, notify, err := newService(storageUser, storageUserRealm, notifier).
		PrepareAuthorization(context.Background(), userID, dto.AuthorizeUserOperation{Realm: "site/admin", LangCode: "en"})
	require.NoError(t, err)
	assert.Equal(t, userID, scopes.UserID)
	assert.Equal(t, "site/admin", scopes.Realm)

	// синхронно ничего не отправлено - только отложенный callback
	assert.Empty(t, notifier.events, "login-alert must not be sent synchronously")
	require.NotNil(t, notify)

	notify(context.Background())
	assert.Equal(t, []string{"user.authorization.success.site/admin"}, notifier.events)
}
