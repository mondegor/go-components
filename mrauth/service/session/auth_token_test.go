package session_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/service/session"
	"github.com/mondegor/go-components/mrauth/service/session/mock"
)

//go:generate mockgen -source=auth_token.go -destination=mock/auth_token.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-sysmess/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth TokenIssuer

const testRealm = "site/admin"

type AuthTokenSuite struct {
	suite.Suite

	ctrl    *gomock.Controller
	ctx     context.Context
	tx      *mock.MockDBTxManager
	storage *mock.MockauthTokenStorage
	issuer  *mock.MockTokenIssuer
	sv      *session.AuthToken
}

func TestAuthTokenSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(AuthTokenSuite))
}

func (s *AuthTokenSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.tx = mock.NewMockDBTxManager(s.ctrl)
	s.storage = mock.NewMockauthTokenStorage(s.ctrl)
	s.issuer = mock.NewMockTokenIssuer(s.ctrl)
	s.sv = session.NewAuthToken(
		s.tx,
		s.storage,
		mrlog.NopLogger(),
		[]session.AuthTokenRealm{{Name: testRealm, TokenIssuer: s.issuer}},
	)
}

// expectTxRunsJob - настраивает txManager так, что он выполняет переданную работу.
func (s *AuthTokenSuite) expectTxRunsJob() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, job func(context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		},
	)
}

func userScopes() dto.UserScopes {
	return dto.UserScopes{
		UserID:    uuid.New(),
		SessionID: 123,
		Realm:     testRealm,
		Kind:      "admin",
		LangCode:  "en",
	}
}

func tokenPair(us dto.UserScopes, hasSignature bool) dto.AuthTokenPair {
	return dto.AuthTokenPair{
		Access:  dto.AccessToken{Token: "access", ExpiresIn: time.Minute, HasSignature: hasSignature},
		Refresh: dto.RefreshToken{Token: "refresh", ExpiresIn: time.Hour},
		UserID:  us.UserID,
		Scopes:  entity.AuthTokenScopes{Realm: us.Realm, UserKind: us.Kind, LangCode: us.LangCode},
	}
}

func (s *AuthTokenSuite) TestCreate_SessionIDRequired() {
	us := userScopes()
	us.SessionID = 0

	_, err := s.sv.Create(s.ctx, us)
	s.Require().Error(err)
}

func (s *AuthTokenSuite) TestCreate_UnknownRealm() {
	us := userScopes()
	us.Realm = "unknown"

	_, err := s.sv.Create(s.ctx, us)
	s.Require().Error(err)
}

func (s *AuthTokenSuite) TestCreate_IssuerError() {
	us := userScopes()
	s.issuer.EXPECT().CreateTokenPair(us).Return(dto.AuthTokenPair{}, errors.New("issuer failed"))

	_, err := s.sv.Create(s.ctx, us)
	s.Require().Error(err)
}

func (s *AuthTokenSuite) TestCreate_JWTStoresRefreshOnly() {
	us := userScopes()
	pair := tokenPair(us, true)
	s.issuer.EXPECT().CreateTokenPair(us).Return(pair, nil)
	s.storage.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rows []entity.AuthToken) error {
			s.Len(rows, 1)

			return nil
		},
	)

	got, err := s.sv.Create(s.ctx, us)
	s.Require().NoError(err)
	s.Equal(pair, got)
}

func (s *AuthTokenSuite) TestCreate_SessionStoresBothTokens() {
	us := userScopes()
	pair := tokenPair(us, false)
	s.issuer.EXPECT().CreateTokenPair(us).Return(pair, nil)
	s.storage.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rows []entity.AuthToken) error {
			s.Len(rows, 2)

			return nil
		},
	)

	_, err := s.sv.Create(s.ctx, us)
	s.Require().NoError(err)
}

func (s *AuthTokenSuite) TestCreate_InsertError() {
	us := userScopes()
	s.issuer.EXPECT().CreateTokenPair(us).Return(tokenPair(us, true), nil)
	s.storage.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(errors.New("insert failed"))

	_, err := s.sv.Create(s.ctx, us)
	s.Require().Error(err)
}

func (s *AuthTokenSuite) TestRecreate_RevokeErrorPassedThrough() {
	wantErr := errors.New("revoke failed")

	s.expectTxRunsJob()
	s.storage.EXPECT().RevokeRefresh(gomock.Any(), "rt", gomock.Any()).
		Return(dto.UserScopes{}, false, wantErr)

	_, err := s.sv.Recreate(s.ctx, "rt")
	s.Require().ErrorIs(err, wantErr)
}

func (s *AuthTokenSuite) TestRecreate_RetriedWithStoredAccess() {
	us := userScopes()

	s.expectTxRunsJob()
	s.storage.EXPECT().RevokeRefresh(gomock.Any(), "rt", gomock.Any()).Return(us, true, nil)
	s.storage.EXPECT().FetchLastEnabledPairBySessionID(gomock.Any(), us.UserID, us.SessionID).Return(
		entity.AuthToken{Token: "old-access", ExpiresAt: time.Now().Add(time.Minute)},
		entity.AuthToken{Token: "old-refresh", ExpiresAt: time.Now().Add(time.Hour)},
		nil,
	)

	got, err := s.sv.Recreate(s.ctx, "rt")
	s.Require().NoError(err)
	s.Equal("old-access", got.Access.Token)
	s.Equal("old-refresh", got.Refresh.Token)
}

func (s *AuthTokenSuite) TestRecreate_RetriedJWTReissuesAccess() {
	us := userScopes()

	s.expectTxRunsJob()
	s.storage.EXPECT().RevokeRefresh(gomock.Any(), "rt", gomock.Any()).Return(us, true, nil)
	s.storage.EXPECT().FetchLastEnabledPairBySessionID(gomock.Any(), us.UserID, us.SessionID).Return(
		entity.AuthToken{}, // access пустой => JWT
		entity.AuthToken{Token: "old-refresh", ExpiresAt: time.Now().Add(time.Hour)},
		nil,
	)
	s.issuer.EXPECT().CreateTokenPair(us).Return(tokenPair(us, true), nil)

	got, err := s.sv.Recreate(s.ctx, "rt")
	s.Require().NoError(err)
	s.Equal("access", got.Access.Token)
	s.True(got.Access.HasSignature)
	s.Equal("old-refresh", got.Refresh.Token)
}

func (s *AuthTokenSuite) TestRecreate_IssuesNewPair() {
	us := userScopes()
	pair := tokenPair(us, true)

	s.expectTxRunsJob()
	s.storage.EXPECT().RevokeRefresh(gomock.Any(), "rt", gomock.Any()).Return(us, false, nil)
	s.issuer.EXPECT().CreateTokenPair(us).Return(pair, nil)
	s.storage.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)

	got, err := s.sv.Recreate(s.ctx, "rt")
	s.Require().NoError(err)
	s.Equal(pair, got)
}

func (s *AuthTokenSuite) TestClose_Success() {
	s.storage.EXPECT().RevokeSessionByRefreshToken(gomock.Any(), "rt").Return(nil)

	s.Require().NoError(s.sv.Close(s.ctx, "rt"))
}

func (s *AuthTokenSuite) TestClose_Error() {
	s.storage.EXPECT().RevokeSessionByRefreshToken(gomock.Any(), "rt").
		Return(sysmesserrors.ErrEventStorageNoRecordFound)

	s.Require().Error(s.sv.Close(s.ctx, "rt"))
}
