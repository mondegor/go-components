package userinfo_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/service/userinfo"
	"github.com/mondegor/go-components/mrauth/service/userinfo/mock"
)

//go:generate mockgen -source=user_info.go -destination=mock/user_info.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager

type UserInfoSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	ctx          context.Context
	txManager    *mock.MockDBTxManager
	userFetcher  *mock.MockuserFetcher
	auth2faFetch *mock.Mockuser2faFetcher
	statFetcher  *mock.MockuserActivityStatFetcher
	realmFetcher *mock.MockuserRealmFetcher
	opFetcher    *mock.MockoperationFetcher
}

func TestUserInfoSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(UserInfoSuite))
}

func (s *UserInfoSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *UserInfoSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.userFetcher = mock.NewMockuserFetcher(s.ctrl)
	s.auth2faFetch = mock.NewMockuser2faFetcher(s.ctrl)
	s.statFetcher = mock.NewMockuserActivityStatFetcher(s.ctrl)
	s.realmFetcher = mock.NewMockuserRealmFetcher(s.ctrl)
	s.opFetcher = mock.NewMockoperationFetcher(s.ctrl)

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()
}

func (s *UserInfoSuite) TestGet() {
	base := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()

	s.userFetcher.EXPECT().
		FetchOne(gomock.Any(), userID).
		Return(entity.User{ID: userID, Email: "u@example.com"}, nil)
	// отсутствие записи 2FA не является ошибкой
	s.auth2faFetch.EXPECT().
		FetchOne(gomock.Any(), userID).
		Return(entity.Auth2FA{}, errors.ErrEventStorageNoRecordFound)
	// статистика есть только для realm 1
	s.statFetcher.EXPECT().
		Fetch(gomock.Any(), userID).
		Return([]entity.UserActivityStat{
			{RealmID: 1, LastLoginIP: netip.MustParseAddr("203.0.113.7"), LastLoggedAt: base.Add(time.Hour)},
		}, nil)
	s.realmFetcher.EXPECT().
		Fetch(gomock.Any(), userID).
		Return([]entity.UserRealm{
			{RealmID: 1, Kind: "admin", CreatedAt: base, UpdatedAt: base},
			{RealmID: 2, Kind: "standard", CreatedAt: base, UpdatedAt: base},
		}, nil)

	// операции личного кабинета попадают в сводку в порядке хранилища, вход - нет
	s.opFetcher.EXPECT().FetchByUserIDAndNames(gomock.Any(), userID, unit.PendingOperationNames()).Return([]secureoperation.SecureOperation{
		s.confirmedOperation(unit.NameAuthorizeUser, nil),
		s.confirmedOperation(unit.NameConfirmChangeEmail, s.emailPayload()),
		s.confirmedOperation(unit.NameConfirmDisable2FA, nil),
	}, nil)

	sv := s.newService(
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

	info, err := sv.Get(s.ctx, userID)
	s.Require().NoError(err)

	s.Equal("u@example.com", info.User.Email)
	s.Require().Len(info.Realms, 2)

	// realm 1: статистика есть, IP резолвится в место
	s.Equal(uint16(1), info.Realms[0].RealmID)
	s.Equal("admin", info.Realms[0].Kind)
	s.Equal("Moscow, RU", info.Realms[0].LastLocation)
	s.Equal(base.Add(time.Hour), info.Realms[0].LastLoggedAt)

	// realm 2: статистики нет - пустое место и нулевое время входа
	s.Equal(uint16(2), info.Realms[1].RealmID)
	s.Empty(info.Realms[1].LastLocation)
	s.True(info.Realms[1].LastLoggedAt.IsZero())

	s.Require().Len(info.PendingOperations, 2)
	s.Equal(operationtype.ChangeEmailConfirm, info.PendingOperations[0].Type)
	s.Equal("new@example.com", info.PendingOperations[0].NewEmail)
	s.Equal(operationtype.Disable2FA, info.PendingOperations[1].Type)
}

// TestGetOperationsError - ошибка чтения операций и нечитаемый payload операции не маскируются
// неполной сводкой.
func (s *UserInfoSuite) TestGetOperationsError() {
	tests := []struct {
		name       string
		operations []secureoperation.SecureOperation
		fetchErr   error
	}{
		{name: "storage error", fetchErr: errors.New("db is down")},
		{name: "broken payload", operations: []secureoperation.SecureOperation{
			s.confirmedOperation(unit.NameConfirmChangeEmail, []byte(`{"new_email":""}`)),
		}},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			userID := uuid.New()

			s.userFetcher.EXPECT().FetchOne(gomock.Any(), userID).Return(entity.User{ID: userID}, nil)
			s.auth2faFetch.EXPECT().FetchOne(gomock.Any(), userID).Return(entity.Auth2FA{}, nil)
			s.statFetcher.EXPECT().Fetch(gomock.Any(), userID).Return(nil, nil)
			s.realmFetcher.EXPECT().Fetch(gomock.Any(), userID).Return(nil, nil)
			s.opFetcher.EXPECT().FetchByUserIDAndNames(gomock.Any(), userID, unit.PendingOperationNames()).Return(tt.operations, tt.fetchErr)

			_, err := s.newService(nil).Get(s.ctx, userID)
			s.Require().Error(err)
		})
	}
}

// confirmedOperation - подтверждённая операция указанного типа с указанным payload.
func (s *UserInfoSuite) confirmedOperation(name string, payload []byte) secureoperation.SecureOperation {
	s.T().Helper()

	op := secureoperation.SecureOperation{
		Token:     "token-" + name,
		Name:      name,
		Payload:   payload,
		Status:    operationstatus.Confirmed,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	s.Require().NoError(secureoperation.WakeUp(&op, nil))

	return op
}

func (s *UserInfoSuite) emailPayload() []byte {
	s.T().Helper()

	raw, err := unit.BuildChangeEmailPayload(dto.ChangeEmailOperation{NewEmail: "new@example.com", Email: "user@example.com"})
	s.Require().NoError(err)

	return raw
}

// newService - собирает сервис на моках набора с указанным резолвером местоположения.
func (s *UserInfoSuite) newService(locationResolver mrauth.LocationResolver) *userinfo.UserInfo {
	return userinfo.New(
		s.txManager,
		s.userFetcher,
		s.auth2faFetch,
		s.statFetcher,
		s.realmFetcher,
		s.opFetcher,
		locationResolver,
	)
}
