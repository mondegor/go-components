package httpv1_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrserver"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/mock"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

//go:generate mockgen -source=auth.go -destination=mock/auth.go -package=mock
//go:generate mockgen -destination=mock/validate.go -package=mock github.com/mondegor/go-components/mrauth/validate RequestParser
//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth RealmRegistry
//go:generate mockgen -destination=mock/mrcore.go -package=mock github.com/mondegor/go-webcore/mrcore Localizer

type AuthSuite struct {
	suite.Suite

	ctrl                  *gomock.Controller
	parser                *mock.MockRequestParser
	sender                *mock.MockResponseSender
	localizer             *mock.MockLocalizer
	realmRegistry         *mock.MockRealmRegistry
	serviceUserInfo       *mock.MockuserInfoService
	useCaseCreateUser     *mock.MockcreateUserUseCase
	useCaseChangeSettings *mock.MockchangeSettingsUseCase
	operationResponse     *mock.MockconfirmOperationResponse

	// ответ, отданный контроллером
	sent any
}

func TestAuthSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(AuthSuite))
}

func (s *AuthSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *AuthSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.parser = mock.NewMockRequestParser(s.ctrl)
	s.sender = mock.NewMockResponseSender(s.ctrl)
	s.localizer = mock.NewMockLocalizer(s.ctrl)
	s.realmRegistry = mock.NewMockRealmRegistry(s.ctrl)
	s.serviceUserInfo = mock.NewMockuserInfoService(s.ctrl)
	s.useCaseCreateUser = mock.NewMockcreateUserUseCase(s.ctrl)
	s.useCaseChangeSettings = mock.NewMockchangeSettingsUseCase(s.ctrl)
	s.operationResponse = mock.NewMockconfirmOperationResponse(s.ctrl)
	s.sent = nil

	s.sender.EXPECT().
		Send(gomock.Any(), http.StatusOK, gomock.Any()).
		DoAndReturn(func(_ http.ResponseWriter, _ int, structure any) error {
			s.sent = structure

			return nil
		}).
		AnyTimes()
}

// newController - собирает контроллер с моками; незадействованные в тестах
// зависимости не заполняются.
func (s *AuthSuite) newController() *httpv1.Auth {
	return httpv1.NewAuth(
		s.parser,
		s.sender,
		nil,
		s.useCaseCreateUser,
		nil,
		nil,
		nil,
		nil,
		nil,
		s.useCaseChangeSettings,
		s.serviceUserInfo,
		s.realmRegistry,
		s.operationResponse,
		nil,
	)
}

// expectRequestSettings - язык и пояс, подобранные парсерами по самому запросу:
// именно они подставляются вместо незаполненных полей.
//
// Заодно проверяется, что подбор идёт по запросу без внутренних заголовков: оставь их
// в цепочке - и вместо окружения клиента подобрались бы настройки из предъявленного
// access-токена.
func (s *AuthSuite) expectRequestSettings(langCode, timeZone string) {
	loc, err := time.LoadLocation(timeZone)
	s.Require().NoError(err)

	requireNoTokenSettings := func(r *http.Request) {
		s.Empty(r.Header.Get(mrserver.HeaderKeyInternalLangCode))
		s.Empty(r.Header.Get(mrserver.HeaderKeyInternalTimeZone))
	}

	s.localizer.EXPECT().Language().Return(langCode).AnyTimes()

	s.parser.EXPECT().
		Localizer(gomock.Any()).
		DoAndReturn(func(r *http.Request) mrcore.Localizer {
			requireNoTokenSettings(r)

			return s.localizer
		}).
		AnyTimes()

	s.parser.EXPECT().
		Location(gomock.Any()).
		DoAndReturn(func(r *http.Request) *time.Location {
			requireNoTokenSettings(r)

			return loc
		}).
		AnyTimes()
}

// responseTimeZone - пояс, в котором контроллер форматирует даты ответа (его отдаёт
// parser.Location). Отличается от UTC намеренно: только так проверяется, что перевод
// в пояс запроса действительно выполняется.
const responseTimeZone = "Europe/Moscow"

// mustLoadLocation - загружает часовой пояс или прерывает тест.
func (s *AuthSuite) mustLoadLocation(name string) *time.Location {
	s.T().Helper()

	loc, err := time.LoadLocation(name)
	s.Require().NoError(err)

	return loc
}

// okUserInfo - информация о пользователе со всеми заполненными полями: ответ сверяется
// целиком, поэтому поле, оставленное нулевым, никем бы здесь не удерживалось.
func okUserInfo() dto.UserInfo {
	return dto.UserInfo{
		User: entity.User{
			ID:        uuid.New(),
			Email:     "user@example.com",
			Phone:     79001234567,
			LangCode:  "ru-RU",
			TimeZone:  "Europe/Moscow",
			Status:    userstatus.Enabled,
			CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
		},
		Auth2FA: entity.Auth2FA{
			UserID:        uuid.New(),
			Type:          auth2fatype.TOTP,
			Secret:        "JBSWY3DPEHPK3PXP",
			LastTOTPStep:  42,
			RecoveryCodes: []string{"code-1", "code-2"},
		},
		Realms: []dto.UserRealmInfo{
			{
				RealmID:      7,
				Kind:         "admin",
				LastLocation: "Moscow, RU",
				LastLoggedAt: time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC),
				CreatedAt:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
			},
		},
	}
}

// wantUserInfoResponse - ответ, ожидаемый от контроллера для указанной информации о пользователе.
//
// Даты записаны литералами в поясе responseTimeZone, а не собраны тем же форматированием,
// что и в контроллере: иначе проверка повторяла бы проверяемое и приняла бы любой формат.
func wantUserInfoResponse(info dto.UserInfo, settingsPending bool) model.UserInfoResponse {
	return model.UserInfoResponse{
		Email:       info.User.Email,
		Phone:       "+79001234567",
		LangCode:    info.User.LangCode,
		TimeZone:    info.User.TimeZone,
		Auth2FAType: info.Auth2FA.Type,
		Realms: []model.UserRealm{
			{
				Name:         "site/admin",
				UserKind:     "admin",
				LastLocation: "Moscow, RU",
				LastLoggedAt: "2026-05-06T10:08:09+03:00",
				CreatedAt:    "2026-01-02T06:04:05+03:00",
				UpdatedAt:    "2026-03-04T08:06:07+03:00",
			},
		},
		Status:          info.User.Status,
		SettingsPending: settingsPending,
	}
}

// ptr - возвращает указатель на значение: необязательные поля моделей запросов - указатели,
// чтобы отсутствующее поле отличалось от присланного пустым.
func ptr[T any](value T) *T {
	return &value
}

// expectValidate - разбор тела запроса отдаёт заранее подготовленную структуру.
func expectValidate[T any](s *AuthSuite, req T) {
	s.parser.EXPECT().
		Validate(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ *http.Request, structPointer any) error {
			*structPointer.(*T) = req

			return nil
		})
}

// TestChangeSettings - присланное поле сохраняется как есть, а отсутствующее означает
// режим "авто" и подбирается по самому запросу; в ответ уходят оба сохранённых значения,
// чтобы клиент применил у себя и подобранное.
func (s *AuthSuite) TestChangeSettings() {
	tests := []struct {
		name         string
		req          model.ChangeSettingsRequest
		wantLangCode string
		wantTimeZone string
	}{
		{
			name:         "both are set explicitly",
			req:          model.ChangeSettingsRequest{LangCode: ptr("en-US"), TimeZone: ptr("Asia/Tokyo")},
			wantLangCode: "en-US",
			wantTimeZone: "Asia/Tokyo",
		},
		{
			// пустое тело: обе настройки в режиме "авто" - пользователь просит
			// определить их по его текущему окружению
			name:         "empty body means auto for both",
			req:          model.ChangeSettingsRequest{},
			wantLangCode: "ru-RU",
			wantTimeZone: "Europe/Moscow",
		},
		{
			name:         "only lang is set, time zone is auto",
			req:          model.ChangeSettingsRequest{LangCode: ptr("en-US")},
			wantLangCode: "en-US",
			wantTimeZone: "Europe/Moscow",
		},
		{
			// смена только языка: пояс подбирается заново по клиенту, а не берётся
			// из токена - иначе "авто" вырождалось бы в повтор уже сохранённого
			name:         "only time zone is set, lang is auto",
			req:          model.ChangeSettingsRequest{TimeZone: ptr("Asia/Tokyo")},
			wantLangCode: "ru-RU",
			wantTimeZone: "Asia/Tokyo",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			userID := uuid.New()

			expectValidate(s, tt.req)
			s.expectRequestSettings("ru-RU", "Europe/Moscow")
			s.parser.EXPECT().UserID(gomock.Any()).Return(userID)
			s.useCaseChangeSettings.EXPECT().
				Execute(gomock.Any(), userID, tt.wantLangCode, tt.wantTimeZone).
				Return(nil)

			s.Require().NoError(s.changeSettings())
			s.Equal(
				model.ChangeSettingsResponse{LangCode: tt.wantLangCode, TimeZone: tt.wantTimeZone},
				s.sent,
			)
		})
	}
}

// TestSignup - язык и пояс в теле запроса не передаются: они определяются по самому
// запросу и в таком виде доезжают до usecase, который фиксирует их в payload операции.
func (s *AuthSuite) TestSignup() {
	s.expectRequestSettings("en-US", "Asia/Tokyo")
	expectValidate(s, model.CreateUserRequest{Realm: "shop", UserEmail: "user@example.com"})

	s.localizer.EXPECT().Translate(gomock.Any()).Return("confirm it")
	s.parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{})
	s.useCaseCreateUser.EXPECT().
		Execute(gomock.Any(), "shop", "en-US", "Asia/Tokyo", "user@example.com", gomock.Any()).
		Return(secureoperation.SecureOperation{}, nil)
	s.operationResponse.EXPECT().
		NewConfirmOperation(gomock.Any(), "confirm it").
		Return(model.WaitingConfirmOperationResponse{})

	s.Require().NoError(s.signup())
}

// TestUserInfoSettingsPending - признак сообщает, что сохранённые в профиле настройки
// ещё не попали в предъявленный access-токен: язык или пояс профиля отличается от
// зафиксированного в области действия этого токена.
//
// Значения токена приходят из внутренних заголовков (parser.LangCode и parser.TimeZoneName),
// которые на авторизованных маршрутах заполняет middleware, а на гостевых срезает.
// Пустое значение означает, что настройка в токене не зафиксирована, - сравнивать не с чем,
// и по ней признак не поднимается.
func (s *AuthSuite) TestUserInfoSettingsPending() {
	tests := []struct {
		name          string
		user          entity.User
		tokenLang     string
		tokenTimeZone string
		want          bool
	}{
		{
			name:          "settings applied",
			user:          entity.User{LangCode: "ru-RU", TimeZone: "Europe/Moscow"},
			tokenLang:     "ru-RU",
			tokenTimeZone: "Europe/Moscow",
			want:          false,
		},
		{
			name:          "time zone changed",
			user:          entity.User{LangCode: "ru-RU", TimeZone: "Asia/Tokyo"},
			tokenLang:     "ru-RU",
			tokenTimeZone: "Europe/Moscow",
			want:          true,
		},
		{
			// смена только языка - основной сценарий: пояс совпадает, но тексты
			// в ответах ещё формируются по языку из токена
			name:          "lang changed, time zone applied",
			user:          entity.User{LangCode: "en-US", TimeZone: "Europe/Moscow"},
			tokenLang:     "ru-RU",
			tokenTimeZone: "Europe/Moscow",
			want:          true,
		},
		{
			name:          "both changed",
			user:          entity.User{LangCode: "en-US", TimeZone: "Asia/Tokyo"},
			tokenLang:     "ru-RU",
			tokenTimeZone: "Europe/Moscow",
			want:          true,
		},
		{
			// язык в токене не зафиксирован - сравнивать не с чем,
			// признак решается одним поясом
			name:          "token lang is not set",
			user:          entity.User{LangCode: "ru-RU", TimeZone: "Europe/Moscow"},
			tokenLang:     "",
			tokenTimeZone: "Europe/Moscow",
			want:          false,
		},
		{
			// пояс в токене не зафиксирован - симметрично языку,
			// признак решается одним языком
			name:          "token time zone is not set",
			user:          entity.User{LangCode: "ru-RU", TimeZone: "Europe/Moscow"},
			tokenLang:     "ru-RU",
			tokenTimeZone: "",
			want:          false,
		},
		{
			name:          "nothing is fixed in the token",
			user:          entity.User{LangCode: "ru-RU", TimeZone: "Europe/Moscow"},
			tokenLang:     "",
			tokenTimeZone: "",
			want:          false,
		},
		{
			name:          "default time zone",
			user:          entity.User{LangCode: "ru-RU", TimeZone: "UTC"},
			tokenLang:     "ru-RU",
			tokenTimeZone: "UTC",
			want:          false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			userID := uuid.New()
			info := okUserInfo()
			info.User.LangCode = tt.user.LangCode
			info.User.TimeZone = tt.user.TimeZone

			s.parser.EXPECT().UserID(gomock.Any()).Return(userID)
			s.parser.EXPECT().Location(gomock.Any()).Return(s.mustLoadLocation(responseTimeZone))
			s.parser.EXPECT().LangCode(gomock.Any()).Return(tt.tokenLang)
			s.parser.EXPECT().TimeZoneName(gomock.Any()).Return(tt.tokenTimeZone)
			s.realmRegistry.EXPECT().NameByID(uint16(7)).Return("site/admin", true)
			s.serviceUserInfo.EXPECT().Get(gomock.Any(), userID).Return(info, nil)

			s.Require().NoError(s.userInfo())

			// ответ сверяется целиком, а не по признаку: иначе поле, переставшее доезжать
			// до клиента, осталось бы незамеченным
			s.Equal(wantUserInfoResponse(info, tt.want), s.sent)
		})
	}
}

// настройки из области действия предъявленного access-токена, которые middleware кладёт
// во внутренние заголовки. Намеренно отличаются от подбираемых по запросу: если обработчик
// перестанет их срезать, подбор вернёт именно их.
const (
	tokenLangCode = "de-DE"
	tokenTimeZone = "America/New_York"
)

// newRequestWithTokenSettings - запрос авторизованного пользователя: с внутренними
// заголовками, как его отдаёт middleware.
func newRequestWithTokenSettings(method, target string) *http.Request {
	r := httptest.NewRequest(method, target, http.NoBody)
	r.Header.Set(mrserver.HeaderKeyInternalLangCode, tokenLangCode)
	r.Header.Set(mrserver.HeaderKeyInternalTimeZone, tokenTimeZone)

	return r
}

// requireRequestNotModified - сам запрос обработчик не трогает: он живёт дольше обработчика,
// его дочитывают трассировщики ответа, и подмена настроек в нём аукнулась бы там.
func (s *AuthSuite) requireRequestNotModified(r *http.Request) {
	s.Equal(tokenLangCode, r.Header.Get(mrserver.HeaderKeyInternalLangCode))
	s.Equal(tokenTimeZone, r.Header.Get(mrserver.HeaderKeyInternalTimeZone))
}

func (s *AuthSuite) changeSettings() error {
	r := newRequestWithTokenSettings(http.MethodPost, "/v1/user/settings")

	err := s.newController().ChangeSettings(httptest.NewRecorder(), r)

	s.requireRequestNotModified(r)

	return err
}

// маршрут регистрации зарегистрирован как PermissionGuestOnly, поэтому внутренние заголовки
// до обработчика не доходят: middleware отвергает запрос с access-токеном и удаляет заголовки,
// присланные клиентом самостоятельно.
func (s *AuthSuite) signup() error {
	return s.newController().Signup(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/v1/signup", http.NoBody),
	)
}

func (s *AuthSuite) userInfo() error {
	return s.newController().UserInfo(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/v1/user", http.NoBody),
	)
}
