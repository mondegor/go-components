package session_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/session"
	"github.com/mondegor/go-components/mrauth/usecase/session/mock"
)

//go:generate mockgen -source=session_open.go -destination=mock/session_open.go -package=mock
//go:generate mockgen -source=session_continue.go -destination=mock/session_continue.go -package=mock
//go:generate mockgen -source=session_close.go -destination=mock/session_close.go -package=mock
//go:generate mockgen -source=session_list.go -destination=mock/session_list.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-sysmess/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrevent.go -package=mock github.com/mondegor/go-sysmess/mrevent Emitter

func okScopes() dto.UserScopes {
	return dto.UserScopes{UserID: uuid.New(), Realm: "site/admin", Kind: "admin", LangCode: "en"}
}

func okPair() dto.AuthTokenPair {
	return dto.AuthTokenPair{
		Access:  dto.AccessToken{Token: "access"},
		Refresh: dto.RefreshToken{Token: "refresh"},
	}
}

func runJob(_ context.Context, job func(context.Context) error, _ ...mrstorage.TxOption) error {
	return job(context.Background())
}

// ----- OpenSession -----

type OpenSessionSuite struct {
	suite.Suite

	ctrl     *gomock.Controller
	ctx      context.Context
	tx       *mock.MockDBTxManager
	session  *mock.MocksessionStorage
	activity *mock.MockuserActivityStatCreator
	create   *mock.MockoperationHandlerCreateUser
	before   *mock.MockoperationHandlerBeforeAuthUser
	creator  *mock.MocktokenCreator
	uc       *session.OpenSession
}

func TestOpenSessionSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(OpenSessionSuite))
}

func (s *OpenSessionSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.tx = mock.NewMockDBTxManager(s.ctrl)
	s.session = mock.NewMocksessionStorage(s.ctrl)
	s.activity = mock.NewMockuserActivityStatCreator(s.ctrl)
	s.create = mock.NewMockoperationHandlerCreateUser(s.ctrl)
	s.before = mock.NewMockoperationHandlerBeforeAuthUser(s.ctrl)
	s.creator = mock.NewMocktokenCreator(s.ctrl)
	s.uc = session.NewOpenSession(s.tx, s.session, s.activity, s.create, s.before, s.creator)
}

func confirmedOp(name string) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Name:    name,
		UserID:  uuid.New(),
		Payload: []byte("payload"),
		Status:  operationstatus.Confirmed,
	}
}

func (s *OpenSessionSuite) TestNotConfirmed() {
	op := secureoperation.SecureOperation{Name: unit.NameConfirmCreateUser, Status: operationstatus.Opened}

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, op)
	s.Require().ErrorIs(err, secureoperation.ErrOperationIsNotConfirmed)
}

func (s *OpenSessionSuite) TestCreateUserHappy() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.create.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.session.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)
	s.activity.EXPECT().InsertOrUpdate(gomock.Any(), gomock.Any()).Return(nil)

	got, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().NoError(err)
	s.Equal(okPair(), got)
}

func (s *OpenSessionSuite) TestAuthorizeUserHappy() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.before.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Return(okScopes(), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.session.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)
	s.activity.EXPECT().InsertOrUpdate(gomock.Any(), gomock.Any()).Return(nil)

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameAuthorizeUser))
	s.Require().NoError(err)
}

func (s *OpenSessionSuite) TestUnknownOperation() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp("unknown.operation"))
	s.Require().Error(err)
}

func (s *OpenSessionSuite) TestHandlerError() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.create.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(dto.UserScopes{}, errors.New("handler failed"))

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().Error(err)
}

func (s *OpenSessionSuite) TestTokenCreatorError() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.create.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(dto.AuthTokenPair{}, errors.New("create failed"))

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().Error(err)
}

func (s *OpenSessionSuite) TestActivityError() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.create.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.session.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)
	s.activity.EXPECT().InsertOrUpdate(gomock.Any(), gomock.Any()).Return(errors.New("activity failed"))

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().Error(err)
}

// ----- ContinueSession -----

type ContinueSessionSuite struct {
	suite.Suite

	ctrl      *gomock.Controller
	ctx       context.Context
	storage   *mock.MockauthTokenStorage
	recreator *mock.MocktokenRecreator
	emitter   *mock.MockEmitter
	uc        *session.ContinueSession
}

func TestContinueSessionSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ContinueSessionSuite))
}

func (s *ContinueSessionSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.storage = mock.NewMockauthTokenStorage(s.ctrl)
	s.recreator = mock.NewMocktokenRecreator(s.ctrl)
	s.emitter = mock.NewMockEmitter(s.ctrl)
	s.uc = session.NewContinueSession(s.storage, s.recreator, s.emitter, mrlog.NopLogger())
}

func (s *ContinueSessionSuite) TestEmptyToken() {
	_, err := s.uc.Execute(s.ctx, "en", "")
	s.Require().Error(err)
}

func (s *ContinueSessionSuite) TestHappy() {
	s.recreator.EXPECT().Recreate(gomock.Any(), "rt").Return(okPair(), nil)

	got, err := s.uc.Execute(s.ctx, "en", "rt")
	s.Require().NoError(err)
	s.Equal(okPair(), got)
}

func (s *ContinueSessionSuite) TestReuseRevokesSession() {
	userID := uuid.New()
	revokedErr := repository.NewTokenAlreadyRevokedError(userID, 123)
	s.recreator.EXPECT().Recreate(gomock.Any(), "rt").Return(dto.AuthTokenPair{}, revokedErr)
	s.storage.EXPECT().RevokeTokensBySessionID(gomock.Any(), userID, uint32(123)).Return(nil)
	s.emitter.EXPECT().Emit(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	_, err := s.uc.Execute(s.ctx, "en", "rt")
	s.Require().ErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

func (s *ContinueSessionSuite) TestNoRecordFound() {
	s.recreator.EXPECT().Recreate(gomock.Any(), "rt").
		Return(dto.AuthTokenPair{}, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err := s.uc.Execute(s.ctx, "en", "rt")
	s.Require().ErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

func (s *ContinueSessionSuite) TestTokenExpired() {
	s.recreator.EXPECT().Recreate(gomock.Any(), "rt").
		Return(dto.AuthTokenPair{}, repository.ErrTokenExpired)

	_, err := s.uc.Execute(s.ctx, "en", "rt")
	s.Require().ErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

func (s *ContinueSessionSuite) TestOtherError() {
	s.recreator.EXPECT().Recreate(gomock.Any(), "rt").
		Return(dto.AuthTokenPair{}, errors.New("db down"))

	_, err := s.uc.Execute(s.ctx, "en", "rt")
	s.Require().Error(err)
	s.NotErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

// ----- CloseSession -----

type CloseSessionSuite struct {
	suite.Suite

	ctrl   *gomock.Controller
	ctx    context.Context
	closer *mock.MocktokenCloser
	uc     *session.CloseSession
}

func TestCloseSessionSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(CloseSessionSuite))
}

func (s *CloseSessionSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.closer = mock.NewMocktokenCloser(s.ctrl)
	s.uc = session.NewCloseSession(s.closer)
}

func (s *CloseSessionSuite) TestEmptyToken() {
	s.Require().Error(s.uc.Execute(s.ctx, ""))
}

func (s *CloseSessionSuite) TestSuccess() {
	s.closer.EXPECT().Close(gomock.Any(), "rt").Return(nil)

	s.Require().NoError(s.uc.Execute(s.ctx, "rt"))
}

func (s *CloseSessionSuite) TestNoRecordFound() {
	s.closer.EXPECT().Close(gomock.Any(), "rt").Return(sysmesserrors.ErrEventStorageNoRecordFound)

	err := s.uc.Execute(s.ctx, "rt")
	s.Require().ErrorIs(err, mrauth.ErrTokenInvalid)
}

func (s *CloseSessionSuite) TestOtherError() {
	s.closer.EXPECT().Close(gomock.Any(), "rt").Return(errors.New("db down"))

	err := s.uc.Execute(s.ctx, "rt")
	s.Require().Error(err)
	s.NotErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

// ----- List -----

type ListSuite struct {
	suite.Suite

	ctrl     *gomock.Controller
	ctx      context.Context
	lister   *mock.MocksessionLister
	opener   *mock.MockopenSessionFetcher
	closer   *mock.MocksessionCloser
	resolver *mock.MockcurrentSessionResolver
	userID   uuid.UUID
	uc       *session.List
}

func TestListSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ListSuite))
}

func (s *ListSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.lister = mock.NewMocksessionLister(s.ctrl)
	s.opener = mock.NewMockopenSessionFetcher(s.ctrl)
	s.closer = mock.NewMocksessionCloser(s.ctrl)
	s.resolver = mock.NewMockcurrentSessionResolver(s.ctrl)
	s.userID = uuid.New()
	s.uc = session.NewList(s.lister, s.opener, s.closer, s.resolver, nil, nil)
}

func (s *ListSuite) TestGetListFiltersAndMaps() {
	// 0xdeadbeef нет среди открытых -> должна быть отброшена; порядок результата следует порядку строк
	rows := []entity.Session{
		{UserID: s.userID, SessionID: 0x1f3bc817, UserAgent: "UA1", LastIP: 0x7f000001}, // 127.0.0.1, текущая
		{UserID: s.userID, SessionID: 0x0000babc, UserAgent: "UA2", LastIP: 0},          // IP=0 -> ""
		{UserID: s.userID, SessionID: 0xdeadbeef, UserAgent: "UA3", LastIP: 0x08080808}, // не открыта
	}
	open := []uint32{0x0000babc, 0x1f3bc817} // отсортирован по возрастанию для BinaryContains

	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID).Return(open, nil)
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 0x1f3bc817}, nil)
	s.lister.EXPECT().FetchListByUserID(gomock.Any(), s.userID).Return(rows, nil)

	got, err := s.uc.GetList(s.ctx, s.userID, "acc")
	s.Require().NoError(err)
	s.Require().Len(got, 2)

	s.Equal(uint32(0x1f3bc817), got[0].SessionID)
	s.True(got[0].IsCurrent) // session_id совпал с текущим
	s.Equal("127.0.0.1", got[0].LastIP)

	s.Equal(uint32(0x0000babc), got[1].SessionID)
	s.False(got[1].IsCurrent)
	s.Empty(got[1].LastIP)
}

func (s *ListSuite) TestGetListEmptyOpenSetSkipsFetch() {
	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID).Return([]uint32{}, nil)
	// resolver.FetchOneByAccessToken и lister.FetchListByUser НЕ должны вызываться (ранний возврат)

	got, err := s.uc.GetList(s.ctx, s.userID, "acc")
	s.Require().NoError(err)
	s.Empty(got)
	s.NotNil(got) // именно пустой слайс, а не nil (для сериализации в [])
}

func (s *ListSuite) TestGetListResolversEnrich() {
	uc := session.NewList(
		s.lister,
		s.opener,
		s.closer,
		s.resolver,
		func(ua string) (string, string) { return "app:" + ua, "dev:" + ua },
		func(ip string) string { return "loc:" + ip },
	)

	rows := []entity.Session{{UserID: s.userID, SessionID: 1, UserAgent: "UA", LastIP: 0x7f000001}}

	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID).Return([]uint32{1}, nil)
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 0}, nil)
	s.lister.EXPECT().FetchListByUserID(gomock.Any(), s.userID).Return(rows, nil)

	got, err := uc.GetList(s.ctx, s.userID, "acc")
	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal("app:UA", got[0].AppName)
	s.Equal("dev:UA", got[0].DeviceName)
	s.Equal("loc:127.0.0.1", got[0].Location)
}

func (s *ListSuite) TestGetListResolverErrorBestEffort() {
	// резолв текущего session_id упал -> is_current=false у всех, но список возвращается
	rows := []entity.Session{{UserID: s.userID, SessionID: 1, UserAgent: "UA", LastIP: 0x7f000001}}

	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID).Return([]uint32{1}, nil)
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{}, errors.New("token revoked"))
	s.lister.EXPECT().FetchListByUserID(gomock.Any(), s.userID).Return(rows, nil)

	got, err := s.uc.GetList(s.ctx, s.userID, "acc")
	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.False(got[0].IsCurrent)
}

func (s *ListSuite) TestGetListOpenFetcherError() {
	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID).Return(nil, errors.New("db down"))

	_, err := s.uc.GetList(s.ctx, s.userID, "acc")
	s.Require().Error(err)
}

func (s *ListSuite) TestGetListListerError() {
	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID).Return([]uint32{1}, nil)
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 1}, nil)
	s.lister.EXPECT().FetchListByUserID(gomock.Any(), s.userID).Return(nil, errors.New("db down"))

	_, err := s.uc.GetList(s.ctx, s.userID, "acc")
	s.Require().Error(err)
}

func (s *ListSuite) TestCloseEmptyInput() {
	// closer.RevokeTokensBySessionIDs НЕ должен вызываться
	s.Require().Error(s.uc.Close(s.ctx, s.userID, nil))
}

func (s *ListSuite) TestCloseSuccess() {
	ids := []uint32{0x1f3bc817, 0x0000babc}
	s.closer.EXPECT().RevokeTokensBySessionIDs(gomock.Any(), s.userID, ids).Return(nil)

	s.Require().NoError(s.uc.Close(s.ctx, s.userID, ids))
}

func (s *ListSuite) TestCloseError() {
	ids := []uint32{1}
	s.closer.EXPECT().RevokeTokensBySessionIDs(gomock.Any(), s.userID, ids).Return(errors.New("db down"))

	s.Require().Error(s.uc.Close(s.ctx, s.userID, ids))
}
