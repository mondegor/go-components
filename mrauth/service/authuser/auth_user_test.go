package authuser_test

import (
	"context"
	stderrors "errors"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/service/authuser"
	"github.com/mondegor/go-components/mrauth/service/authuser/mock"
	"github.com/mondegor/go-components/mrauth/service/realm"
)

//go:generate mockgen -source=auth_user.go -destination=mock/auth_user.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrnotifier.go -package=mock github.com/mondegor/go-components/mrnotifier NoteProducer

type AuthUserSuite struct {
	suite.Suite

	ctrl             *gomock.Controller
	ctx              context.Context
	txManager        *mock.MockDBTxManager
	storageUser      *mock.MockuserStorage
	storageUserRealm *mock.MockuserRealmStorage
	notifierAPI      *mock.MockNoteProducer
	svc              *authuser.Service
}

func TestAuthUserSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(AuthUserSuite))
}

func (s *AuthUserSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.storageUser = mock.NewMockuserStorage(s.ctrl)
	s.storageUserRealm = mock.NewMockuserRealmStorage(s.ctrl)
	s.notifierAPI = mock.NewMockNoteProducer(s.ctrl)

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()

	s.svc = authuser.New(
		s.txManager,
		s.storageUser,
		s.storageUserRealm,
		realm.New([]realm.Realm{{ID: 1, Name: "site/admin"}}),
		s.notifierAPI,
		mrlog.NopLogger(),
	)
}

// expectNotices - фиксирует ожидаемую последовательность ключей уведомлений;
// пустой список означает, что уведомления не отправляются вовсе.
func (s *AuthUserSuite) expectNotices(keys ...string) {
	if len(keys) == 0 {
		s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		return
	}

	calls := make([]any, 0, len(keys))
	for _, key := range keys {
		calls = append(calls, s.notifierAPI.EXPECT().Send(gomock.Any(), key, gomock.Any()).Return(nil))
	}

	gomock.InOrder(calls...)
}

func newCreateIn() dto.CreateUserOperation {
	return dto.CreateUserOperation{
		Realm:    "site/admin",
		UserKind: "admin",
		LangCode: "en",
		TimeZone: "Europe/Moscow",
		Email:    "user@example.com",
	}
}

// новый email (userID=Nil, пользователь не найден) -> создаётся пользователь и привязка к realm,
// возвращается сгенерированный id, отправляются оба уведомления о регистрации.
func (s *AuthUserSuite) TestResolveUserNewEmail() {
	registeredIP := mrtype.NewIP(netip.MustParseAddr("203.0.113.7"))

	in := newCreateIn()
	in.RegisteredIP = registeredIP

	var insertedUser entity.ExtendedUser

	s.storageUser.EXPECT().
		FetchOneByLogin(gomock.Any(), gomock.Any()).
		Return(entity.User{}, errors.ErrEventStorageNoRecordFound)
	s.storageUser.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, row entity.ExtendedUser) error {
			insertedUser = row

			return nil
		})
	s.storageUserRealm.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)
	s.expectNotices("user.registration.success.site.admin", "user.was.registered")

	userID, err := s.svc.ResolveUser(s.ctx, uuid.Nil, in)
	s.Require().NoError(err)
	s.NotEqual(uuid.Nil, userID)

	s.Equal(userID, insertedUser.ID)
	s.Equal(registeredIP, insertedUser.RegisteredIP, "IP регистрации фиксируется у нового пользователя")
}

// чистый ретрай того же токена (userID=Nil): пользователь и привязка к realm уже зафиксированы
// предыдущей попыткой -> повторная привязка даёт дубль (уже в realm), ничего не создаётся,
// уведомления не шлются, возвращается существующий id.
func (s *AuthUserSuite) TestResolveUserRetryExistingUser() {
	existingID := uuid.New()

	s.storageUser.EXPECT().
		FetchOneByLogin(gomock.Any(), gomock.Any()).
		Return(entity.User{ID: existingID, Email: "user@example.com"}, nil)
	// existing user must not be re-inserted
	s.storageUser.EXPECT().Insert(gomock.Any(), gomock.Any()).Times(0)
	// realm binding already committed in the original transaction
	s.storageUserRealm.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		Return(errors.ErrEventRecordAlreadyExists)
	// registration notifications already sent on the original attempt
	s.expectNotices()

	userID, err := s.svc.ResolveUser(s.ctx, uuid.Nil, newCreateIn())
	s.Require().NoError(err)
	s.Equal(existingID, userID)
}

// пользователь с этим email уже есть (userID=Nil), но привязки к нужному realm нет (параллельный
// кросс-realm signup или внешнее создание email в зазоре) -> привязка достраивается идемпотентно,
// шлётся уведомление о регистрации в realm (юзеру), пользователь не пересоздаётся,
// возвращается существующий id.
func (s *AuthUserSuite) TestResolveUserExistingUserMissingRealmBinding() {
	existingID := uuid.New()

	var boundRealm entity.UserRealm

	s.storageUser.EXPECT().
		FetchOneByLogin(gomock.Any(), gomock.Any()).
		Return(entity.User{ID: existingID, Email: "user@example.com"}, nil)
	s.storageUser.EXPECT().Insert(gomock.Any(), gomock.Any()).Times(0)
	// missing realm binding must be created
	s.storageUserRealm.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, row entity.UserRealm) error {
			boundRealm = row

			return nil
		})
	s.expectNotices("user.registration.success.site.admin")

	userID, err := s.svc.ResolveUser(s.ctx, uuid.Nil, newCreateIn())
	s.Require().NoError(err)
	s.Equal(existingID, userID)
	s.Equal(existingID, boundRealm.UserID)
}

// известный пользователь (userID!=Nil, привязка к новому realm): поиск по email не выполняется,
// создаётся только привязка к realm, шлётся уведомление о регистрации в realm (юзеру),
// возвращается тот же id.
func (s *AuthUserSuite) TestResolveUserKnownUserBindsToRealm() {
	knownID := uuid.New()

	var boundRealm entity.UserRealm

	// FetchOneByLogin must not be called
	s.storageUser.EXPECT().FetchOneByLogin(gomock.Any(), gomock.Any()).Times(0)
	// known user must not be inserted
	s.storageUser.EXPECT().Insert(gomock.Any(), gomock.Any()).Times(0)
	s.storageUserRealm.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, row entity.UserRealm) error {
			boundRealm = row

			return nil
		})
	s.expectNotices("user.registration.success.site.admin")

	userID, err := s.svc.ResolveUser(s.ctx, knownID, newCreateIn())
	s.Require().NoError(err)
	s.Equal(knownID, userID)
	s.Equal(knownID, boundRealm.UserID)
}

// известный пользователь (userID!=Nil), уже привязанный к realm: повторная привязка даёт дубль,
// трактуется как успех - ничего не создаётся, уведомления не шлются, возвращается тот же id.
func (s *AuthUserSuite) TestResolveUserKnownUserAlreadyInRealm() {
	knownID := uuid.New()

	s.storageUserRealm.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		Return(errors.ErrEventRecordAlreadyExists)
	s.expectNotices()

	userID, err := s.svc.ResolveUser(s.ctx, knownID, newCreateIn())
	s.Require().NoError(err)
	s.Equal(knownID, userID)
}

// жёсткая ошибка поиска по email (не "не найдено") пробрасывается, пользователь не создаётся.
func (s *AuthUserSuite) TestResolveUserLookupError() {
	s.storageUser.EXPECT().
		FetchOneByLogin(gomock.Any(), gomock.Any()).
		Return(entity.User{}, stderrors.New("db is down"))
	s.storageUser.EXPECT().Insert(gomock.Any(), gomock.Any()).Times(0)
	s.storageUserRealm.EXPECT().Insert(gomock.Any(), gomock.Any()).Times(0)

	_, err := s.svc.ResolveUser(s.ctx, uuid.Nil, newCreateIn())
	s.Require().Error(err)
}

// PrepareAuthorization сам НЕ шлёт login-alert: он возвращает scopes и отложенный callback,
// а уведомление user.authorization.success.<realm> уходит только при вызове callback'а.
func (s *AuthUserSuite) TestPrepareAuthorizationDefersRealmSpecificNotice() {
	userID := uuid.New()

	s.storageUser.EXPECT().
		FetchOne(gomock.Any(), userID).
		Return(entity.User{ID: userID, Email: "user@example.com", LangCode: "en"}, nil)
	s.storageUserRealm.EXPECT().
		FetchOne(gomock.Any(), userID, uint16(1)).
		Return(entity.UserRealm{UserID: userID, RealmID: 1, Kind: "admin"}, nil)

	notified := false

	s.notifierAPI.EXPECT().
		Send(gomock.Any(), "user.authorization.success.site.admin", gomock.Any()).
		DoAndReturn(func(context.Context, string, map[string]any) error {
			notified = true

			return nil
		})

	scopes, notify, err := s.svc.PrepareAuthorization(
		s.ctx,
		userID,
		dto.AuthorizeUserOperation{Realm: "site/admin", LangCode: "en"},
	)
	s.Require().NoError(err)
	s.Equal(userID, scopes.UserID)
	s.Equal("site/admin", scopes.Realm)

	// синхронно ничего не отправлено - только отложенный callback
	s.False(notified, "login-alert must not be sent synchronously")
	s.Require().NotNil(notify)

	notify(s.ctx)
	s.True(notified)
}
