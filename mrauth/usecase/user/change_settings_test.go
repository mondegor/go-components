package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/usecase/user"
	"github.com/mondegor/go-components/mrauth/usecase/user/mock"
)

//go:generate mockgen -source=change_settings.go -destination=mock/change_settings.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager

type ChangeSettingsSuite struct {
	suite.Suite

	ctrl             *gomock.Controller
	ctx              context.Context
	txManager        *mock.MockDBTxManager
	storage          *mock.MockuserSettingsStorage
	storageAuthToken *mock.MockauthTokenSettingsStorage
	uc               *user.ChangeSettings
}

func TestChangeSettingsSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ChangeSettingsSuite))
}

func (s *ChangeSettingsSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.storage = mock.NewMockuserSettingsStorage(s.ctrl)
	s.storageAuthToken = mock.NewMockauthTokenSettingsStorage(s.ctrl)
	s.uc = user.NewChangeSettings(s.txManager, s.storage, s.storageAuthToken)
}

// expectPassThroughTx - транзакция выполняет переданное задание как есть.
func (s *ChangeSettingsSuite) expectPassThroughTx() {
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		})
}

// TestExecute - настройки уходят в профиль и в область действия refresh токенов
// одними и теми же значениями: разойдись они, продление сессии вернуло бы прежние.
//
// Сохраняется ровно то, что прислано: приводить значения usecase не должен,
// на границе ввода принимаются только язык и пояс, зарегистрированные приложением.
func (s *ChangeSettingsSuite) TestExecute() {
	userID := uuid.New()

	s.expectPassThroughTx()
	s.storage.EXPECT().
		UpdateSettings(gomock.Any(), entity.UserSettings{UserID: userID, LangCode: "ja-JP", TimeZone: "Asia/Tokyo"}).
		Return(nil)
	s.storageAuthToken.EXPECT().
		UpdateScopesSettings(gomock.Any(), userID, "ja-JP", "Asia/Tokyo").
		Return(nil)

	s.Require().NoError(s.uc.Execute(s.ctx, userID, "ja-JP", "Asia/Tokyo"))
}

// TestExecuteStorageError - ошибка сохранения профиля доходит до вызывающего,
// а область действия токенов при этом не трогается.
func (s *ChangeSettingsSuite) TestExecuteStorageError() {
	errStorage := errors.New("storage is unavailable")

	s.expectPassThroughTx()
	s.storage.EXPECT().UpdateSettings(gomock.Any(), gomock.Any()).Return(errStorage)
	s.storageAuthToken.EXPECT().UpdateScopesSettings(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := s.uc.Execute(s.ctx, uuid.New(), "ru-RU", "Europe/Moscow")
	s.Require().ErrorIs(err, errStorage)
}

// TestExecuteAuthTokenStorageError - ошибка обновления области действия токенов доходит
// до вызывающего: успешный ответ при неперенесённых в токены настройках означал бы,
// что признак "настройки ещё не применены" у клиента уже не погаснет.
func (s *ChangeSettingsSuite) TestExecuteAuthTokenStorageError() {
	errStorage := errors.New("storage is unavailable")

	s.expectPassThroughTx()
	s.storage.EXPECT().UpdateSettings(gomock.Any(), gomock.Any()).Return(nil)
	s.storageAuthToken.EXPECT().UpdateScopesSettings(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errStorage)

	err := s.uc.Execute(s.ctx, uuid.New(), "ru-RU", "Europe/Moscow")
	s.Require().ErrorIs(err, errStorage)
}
