package get_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-sysmess/errors"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/component/get"
	"github.com/mondegor/go-components/mrauth/component/get/mock"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/repository"
)

const allowedRealm = "site/admin"

type UserProviderSuite struct {
	suite.Suite

	ctrl    *gomock.Controller
	ctx     context.Context
	storage *mock.MockAuthTokenFetcher
	rights  *mock.MockRightsGetter
	co      *get.UserProvider
}

func TestUserProviderSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(UserProviderSuite))
}

func (s *UserProviderSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.storage = mock.NewMockAuthTokenFetcher(s.ctrl)
	s.rights = mock.NewMockRightsGetter(s.ctrl)
	s.co = get.New(s.storage, s.rights, []string{allowedRealm})
}

func (s *UserProviderSuite) TestEmptyToken() {
	_, err := s.co.UserByToken(s.ctx, "")
	s.Require().Error(err)
}

func (s *UserProviderSuite) TestNoRecordFound() {
	s.storage.EXPECT().FetchOneByAccessToken(gomock.Any(), "tok").
		Return(dto.UserScopes{}, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err := s.co.UserByToken(s.ctx, "tok")
	s.Require().ErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

func (s *UserProviderSuite) TestTokenExpired() {
	s.storage.EXPECT().FetchOneByAccessToken(gomock.Any(), "tok").
		Return(dto.UserScopes{}, repository.ErrTokenExpired)

	_, err := s.co.UserByToken(s.ctx, "tok")
	s.Require().ErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

func (s *UserProviderSuite) TestOtherError() {
	s.storage.EXPECT().FetchOneByAccessToken(gomock.Any(), "tok").
		Return(dto.UserScopes{}, errors.New("db down"))

	_, err := s.co.UserByToken(s.ctx, "tok")
	s.Require().Error(err)
	s.NotErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

func (s *UserProviderSuite) TestRealmNotAllowed() {
	s.storage.EXPECT().FetchOneByAccessToken(gomock.Any(), "tok").
		Return(dto.UserScopes{Realm: "other"}, nil)

	_, err := s.co.UserByToken(s.ctx, "tok")
	s.Require().ErrorIs(err, sysmesserrors.ErrAccessForbidden)
}

func (s *UserProviderSuite) TestSuccess() {
	userID := uuid.New()
	s.storage.EXPECT().FetchOneByAccessToken(gomock.Any(), "tok").Return(
		dto.UserScopes{UserID: userID, Realm: allowedRealm, Kind: "admin", LangCode: "en"},
		nil,
	)
	s.rights.EXPECT().Rights(allowedRealm + "/admin").Return(nil)

	got, err := s.co.UserByToken(s.ctx, "tok")
	s.Require().NoError(err)
	s.Equal([16]byte(userID), got.ID())
	s.Equal(allowedRealm+"/admin", got.Group())
	s.Equal("en", got.LangCode())
}
