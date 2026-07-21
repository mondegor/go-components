package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/usecase/user"
	"github.com/mondegor/go-components/mrauth/usecase/user/mock"
)

//go:generate mockgen -source=apply_settings.go -destination=mock/apply_settings.go -package=mock

type ApplySettingsSuite struct {
	suite.Suite

	ctrl             *gomock.Controller
	ctx              context.Context
	storage          *mock.MockuserSettingsStorage
	langResolver     *mock.MocklangResolver
	timeZoneResolver *mock.MocktimeZoneResolver
	uc               *user.ApplySettings
}

func TestApplySettingsSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApplySettingsSuite))
}

func (s *ApplySettingsSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.storage = mock.NewMockuserSettingsStorage(s.ctrl)
	s.langResolver = mock.NewMocklangResolver(s.ctrl)
	s.timeZoneResolver = mock.NewMocktimeZoneResolver(s.ctrl)
	s.uc = user.NewApplySettings(s.storage, s.langResolver, s.timeZoneResolver)
}

// TestExecute - сохраняется и возвращается именно то, что вернули резолверы,
// а не то, что прислал клиент: язык приводится к поддерживаемому приложением так же,
// как пояс приводится к зарегистрированному.
func (s *ApplySettingsSuite) TestExecute() {
	userID := uuid.New()
	requested := dto.TimeZoneInfo{Name: "Asia/Unknown", Offset: 9 * time.Hour}

	// аргументы резолверов зафиксированы: в них уходит запрос клиента без изменений
	s.langResolver.EXPECT().Resolve("ja").Return("ja-JP")
	s.timeZoneResolver.EXPECT().Resolve(requested).Return("Asia/Tokyo")
	s.storage.EXPECT().
		UpdateSettings(gomock.Any(), entity.UserSettings{UserID: userID, LangCode: "ja-JP", TimeZone: "Asia/Tokyo"}).
		Return(nil)

	got, err := s.uc.Execute(s.ctx, userID, dto.UserSettings{LangCode: "ja", TimeZone: requested})
	s.Require().NoError(err)

	// вернуться должно ровно то, что ушло в хранилище
	s.Equal("ja-JP", got.LangCode)
	s.Equal("Asia/Tokyo", got.TimeZone)
}

func (s *ApplySettingsSuite) TestExecuteStorageError() {
	errStorage := errors.New("storage is unavailable")

	s.langResolver.EXPECT().Resolve(gomock.Any()).Return("ru-RU")
	s.timeZoneResolver.EXPECT().Resolve(gomock.Any()).Return("UTC")
	s.storage.EXPECT().UpdateSettings(gomock.Any(), gomock.Any()).Return(errStorage)

	got, err := s.uc.Execute(
		s.ctx,
		uuid.New(),
		dto.UserSettings{LangCode: "ru-RU", TimeZone: dto.TimeZoneInfo{Name: "Europe/Moscow"}},
	)

	s.Require().ErrorIs(err, errStorage)
	s.Equal(dto.UserSettingsApplied{}, got)
}
