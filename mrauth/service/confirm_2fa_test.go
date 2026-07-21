package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/service"
	"github.com/mondegor/go-components/mrauth/service/mock"
)

//go:generate mockgen -source=confirm_2fa.go -destination=mock/confirm_2fa.go -package=mock

type FactoryConfirm2FASuite struct {
	suite.Suite

	ctrl          *gomock.Controller
	ctx           context.Context
	userStorage   *mock.MockuserStorage
	user2faStorag *mock.Mockuser2faStorage
	actionFactory *mock.MockfactoryConfirmAction2FA
	svc           *service.FactoryConfirm2FA
}

func TestFactoryConfirm2FASuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(FactoryConfirm2FASuite))
}

func (s *FactoryConfirm2FASuite) SetupSubTest() {
	s.SetupTest()
}

func (s *FactoryConfirm2FASuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.userStorage = mock.NewMockuserStorage(s.ctrl)
	s.user2faStorag = mock.NewMockuser2faStorage(s.ctrl)
	s.actionFactory = mock.NewMockfactoryConfirmAction2FA(s.ctrl)
	s.svc = service.NewFactoryConfirm2FA(s.userStorage, s.user2faStorag, s.actionFactory)
}

func (s *FactoryConfirm2FASuite) TestCreateByUserLogin() {
	login := contactaddress.NewEmail("user@example.com")

	s.Run("user not found - error", func() {
		s.userStorage.EXPECT().
			FetchOneByLogin(gomock.Any(), login).
			Return(entity.User{}, errors.ErrEventStorageNoRecordFound)

		_, err := s.svc.CreateByUserLogin(s.ctx, login)
		s.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
	})

	s.Run("existing user with 2fa - action populated", func() {
		userID := uuid.New()

		s.userStorage.EXPECT().
			FetchOneByLogin(gomock.Any(), login).
			Return(entity.User{ID: userID, Email: "user@example.com", Status: userstatus.Enabled}, nil)
		s.user2faStorag.EXPECT().
			FetchOne(gomock.Any(), userID).
			Return(entity.Auth2FA{UserID: userID, Type: auth2fatype.TOTP, Secret: "secret"}, nil)
		s.actionFactory.EXPECT().
			Create(auth2fatype.TOTP, "secret").
			Return(secureoperation.ConfirmAction{Method: confirmmethod.TOTP}, nil)

		got, err := s.svc.CreateByUserLogin(s.ctx, login)
		s.Require().NoError(err)
		s.Equal(userID, got.ID)
		s.Equal(confirmmethod.TOTP, got.Action2FA.Method)
	})

	s.Run("existing user without 2fa - empty action", func() {
		userID := uuid.New()

		s.userStorage.EXPECT().
			FetchOneByLogin(gomock.Any(), login).
			Return(entity.User{ID: userID, Email: "user@example.com", Status: userstatus.Enabled}, nil)
		// отсутствие записи 2FA не является ошибкой, фабрика действия при этом не вызывается
		s.user2faStorag.EXPECT().
			FetchOne(gomock.Any(), userID).
			Return(entity.Auth2FA{}, errors.ErrEventStorageNoRecordFound)

		got, err := s.svc.CreateByUserLogin(s.ctx, login)
		s.Require().NoError(err)
		s.Equal(userID, got.ID)
		s.Equal(confirmmethod.Enum(0), got.Action2FA.Method)
	})

	s.Run("disabled user - error", func() {
		s.userStorage.EXPECT().
			FetchOneByLogin(gomock.Any(), login).
			Return(entity.User{ID: uuid.New(), Status: userstatus.Disabled}, nil)

		_, err := s.svc.CreateByUserLogin(s.ctx, login)
		s.Require().Error(err)
	})
}
