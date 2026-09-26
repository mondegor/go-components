package notify_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/service/notify"
	"github.com/mondegor/go-components/mrauth/service/notify/mock"
)

//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth Notifier
//go:generate mockgen -source=user_email.go -destination=mock/user_email.go -package=mock

type UserEmailNotifierSuite struct {
	suite.Suite

	ctrl        *gomock.Controller
	ctx         context.Context
	notifierAPI *mock.MockNotifier
	storageUser *mock.MockuserFetcher
	svc         *notify.UserEmailNotifier
}

func TestUserEmailNotifierSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(UserEmailNotifierSuite))
}

func (s *UserEmailNotifierSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.notifierAPI = mock.NewMockNotifier(s.ctrl)
	s.storageUser = mock.NewMockuserFetcher(s.ctrl)
	s.svc = notify.NewUserEmailNotifier(s.notifierAPI, s.storageUser)
}

func (s *UserEmailNotifierSuite) TestAddressPassedThrough() {
	props := map[string]any{"to": "user@example.com", "lang": "ru"}

	s.storageUser.EXPECT().FetchOne(gomock.Any(), gomock.Any()).Times(0)
	s.notifierAPI.EXPECT().Send(s.ctx, "some.key", props).Return(nil)

	s.Require().NoError(s.svc.Send(s.ctx, "some.key", props))
}

func (s *UserEmailNotifierSuite) TestUserIDReplacedWithEmail() {
	userID := uuid.New()
	props := map[string]any{"to": userID, "remaining": 2}

	s.storageUser.EXPECT().FetchOne(s.ctx, userID).Return(entity.User{ID: userID, Email: "user@example.com"}, nil)
	s.notifierAPI.EXPECT().
		Send(s.ctx, "some.key", map[string]any{"to": "user@example.com", "remaining": 2}).
		Return(nil)

	s.Require().NoError(s.svc.Send(s.ctx, "some.key", props))
	s.Equal(userID, props["to"]) // props вызывающего не изменены
}

func (s *UserEmailNotifierSuite) TestFetchUserError() {
	userID := uuid.New()
	errFetch := errors.New("fetch failed")

	s.storageUser.EXPECT().FetchOne(s.ctx, userID).Return(entity.User{}, errFetch)
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	s.Require().ErrorIs(s.svc.Send(s.ctx, "some.key", map[string]any{"to": userID}), errFetch)
}
