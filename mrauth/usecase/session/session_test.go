package session_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/session"
	"github.com/mondegor/go-components/mrauth/usecase/session/mock"
)

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
	s.activity = mock.NewMockuserActivityStatCreator(s.ctrl)
	s.create = mock.NewMockoperationHandlerCreateUser(s.ctrl)
	s.before = mock.NewMockoperationHandlerBeforeAuthUser(s.ctrl)
	s.creator = mock.NewMocktokenCreator(s.ctrl)
	s.uc = session.NewOpenSession(s.tx, s.activity, s.create, s.before, s.creator)
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

	_, err := s.uc.Execute(s.ctx, mrtype.DetailedIP{}, op)
	s.Require().ErrorIs(err, secureoperation.ErrOperationIsNotConfirmed)
}

func (s *OpenSessionSuite) TestCreateUserHappy() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.create.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.activity.EXPECT().InsertOrUpdate(gomock.Any(), gomock.Any()).Return(nil)

	got, err := s.uc.Execute(s.ctx, mrtype.DetailedIP{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().NoError(err)
	s.Equal(okPair(), got)
}

func (s *OpenSessionSuite) TestAuthorizeUserHappy() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.before.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Return(okScopes(), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.activity.EXPECT().InsertOrUpdate(gomock.Any(), gomock.Any()).Return(nil)

	_, err := s.uc.Execute(s.ctx, mrtype.DetailedIP{}, confirmedOp(unit.NameAuthorizeUser))
	s.Require().NoError(err)
}

func (s *OpenSessionSuite) TestUnknownOperation() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)

	_, err := s.uc.Execute(s.ctx, mrtype.DetailedIP{}, confirmedOp("unknown.operation"))
	s.Require().Error(err)
}

func (s *OpenSessionSuite) TestHandlerError() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.create.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(dto.UserScopes{}, errors.New("handler failed"))

	_, err := s.uc.Execute(s.ctx, mrtype.DetailedIP{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().Error(err)
}

func (s *OpenSessionSuite) TestTokenCreatorError() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.create.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(dto.AuthTokenPair{}, errors.New("create failed"))

	_, err := s.uc.Execute(s.ctx, mrtype.DetailedIP{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().Error(err)
}

func (s *OpenSessionSuite) TestActivityError() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.create.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.activity.EXPECT().InsertOrUpdate(gomock.Any(), gomock.Any()).Return(errors.New("activity failed"))

	_, err := s.uc.Execute(s.ctx, mrtype.DetailedIP{}, confirmedOp(unit.NameConfirmCreateUser))
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
	s.storage.EXPECT().RevokeSession(gomock.Any(), userID, uint32(123)).Return(nil)
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
	s.Require().ErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

func (s *CloseSessionSuite) TestOtherError() {
	s.closer.EXPECT().Close(gomock.Any(), "rt").Return(errors.New("db down"))

	err := s.uc.Execute(s.ctx, "rt")
	s.Require().Error(err)
	s.NotErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}
