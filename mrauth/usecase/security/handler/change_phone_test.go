package handler_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/service/notify"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler/mock"
)

//go:generate mockgen -source=change_phone.go -destination=mock/change_phone.go -package=mock

type ChangePhoneSuite struct {
	suite.Suite

	ctrl        *gomock.Controller
	ctx         context.Context
	txManager   *mock.MockDBTxManager
	storage     *mock.MockuserPhoneChanger
	notifierAPI *mock.MockNotifier
	uc          *handler.ChangePhone
}

func TestChangePhoneSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ChangePhoneSuite))
}

func (s *ChangePhoneSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.storage = mock.NewMockuserPhoneChanger(s.ctrl)
	s.notifierAPI = mock.NewMockNotifier(s.ctrl)

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()

	s.uc = handler.NewChangePhone(
		s.txManager,
		s.storage,
		s.notifierAPI,
		notify.NewActorProps(func(string) (string, string) {
			return "TestApp", "TestDevice"
		}),
	)
}

func (s *ChangePhoneSuite) payload() []byte {
	s.T().Helper()

	raw, err := unit.BuildChangePhonePayload(dto.ChangePhoneOperation{NewPhone: 79991234567, Email: "user@example.com"})
	s.Require().NoError(err)

	return raw
}

// TestExecute - телефон меняется на новый, уведомление о смене уходит на email пользователя
// с контекстом клиента (время, IP, устройство).
func (s *ChangePhoneSuite) TestExecute() {
	userID := uuid.New()
	actor := dto.ActorMeta{
		UserID:   userID,
		ClientIP: mrtype.NewIP(netip.MustParseAddr("192.0.2.10")),
	}

	gomock.InOrder(
		s.storage.EXPECT().UpdatePhone(gomock.Any(), userID, uint64(79991234567)).Return(nil),
		s.notifierAPI.EXPECT().
			Send(gomock.Any(), "user.phone.changed", gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, props map[string]any) error {
				s.Equal("user@example.com", props["to"])
				s.Equal("192.0.2.10", props["ip"])
				s.Equal("TestApp, TestDevice", props["device"])
				s.Contains(props, "occurredAt")

				return nil
			}),
	)

	s.Require().NoError(s.uc.Execute(s.ctx, actor, s.payload()))
}
