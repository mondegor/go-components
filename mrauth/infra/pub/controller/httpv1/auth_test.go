package httpv1_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrtype"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrserver"
	"github.com/mondegor/go-webcore/mrserver/mrresp"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/mock"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
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
	useCaseAuthUser       *mock.MockauthUserUseCase
	useCaseConfirmOp      *mock.MockconfirmOperationUseCase
	useCaseOpenSession    *mock.MockopenSessionUseCase
	useCaseContinueSess   *mock.MockcontinueSessionUseCase
	refreshCookie         *mock.MockcookieValueService
	useCaseChangeSettings *mock.MockchangeSettingsUseCase
	operationResponse     *mock.MockconfirmOperationResponse

	// sessionLimitRetryAfter - период фоновой чистки лишних сессий, с которым собирается
	// контроллер (по умолчанию нулевой)
	sessionLimitRetryAfter time.Duration

	// rec - ответ обработчика: через него проверяются заголовки, выставленные напрямую в w
	rec *httptest.ResponseRecorder

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
	s.useCaseContinueSess = mock.NewMockcontinueSessionUseCase(s.ctrl)
	s.refreshCookie = mock.NewMockcookieValueService(s.ctrl)
	s.useCaseCreateUser = mock.NewMockcreateUserUseCase(s.ctrl)
	s.useCaseAuthUser = mock.NewMockauthUserUseCase(s.ctrl)
	s.useCaseConfirmOp = mock.NewMockconfirmOperationUseCase(s.ctrl)
	s.useCaseOpenSession = mock.NewMockopenSessionUseCase(s.ctrl)
	s.useCaseChangeSettings = mock.NewMockchangeSettingsUseCase(s.ctrl)
	s.operationResponse = mock.NewMockconfirmOperationResponse(s.ctrl)
	s.sessionLimitRetryAfter = 0
	s.rec = httptest.NewRecorder()
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
		s.refreshCookie,
		s.useCaseCreateUser,
		s.useCaseAuthUser,
		s.useCaseConfirmOp,
		s.useCaseOpenSession,
		s.useCaseContinueSess,
		nil,
		s.useCaseChangeSettings,
		s.serviceUserInfo,
		s.realmRegistry,
		s.operationResponse,
		s.sessionLimitRetryAfter,
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

// wantUserInfoResponse - ответ, ожидаемый от контроллера для указанной информации
// о пользователе, где realmName - имя, под которым отдаётся единственный realm.
//
// Даты записаны литералами в поясе responseTimeZone, а не собраны тем же форматированием,
// что и в контроллере: иначе проверка повторяла бы проверяемое и приняла бы любой формат.
func wantUserInfoResponse(info dto.UserInfo, realmName string) model.UserInfoResponse {
	return model.UserInfoResponse{
		Email:       info.User.Email,
		Phone:       "+79001234567",
		LangCode:    info.User.LangCode,
		TimeZone:    info.User.TimeZone,
		Auth2FAType: info.Auth2FA.Type,
		// у okUserInfo два аварийных кода; число записано литералом, а не len(...),
		// чтобы проверка не повторяла подсчёт, выполняемый контроллером
		RecoveryCodesLeft: ptr(2),
		Realms: []model.UserRealm{
			{
				Name:         realmName,
				UserKind:     "admin",
				LastLocation: "Moscow, RU",
				LastLoggedAt: "2026-05-06T10:08:09+03:00",
				CreatedAt:    "2026-01-02T06:04:05+03:00",
				UpdatedAt:    "2026-03-04T08:06:07+03:00",
			},
		},
		Status: info.User.Status,
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
		Execute(gomock.Any(), "shop", "en-US", "Asia/Tokyo", contactaddress.NewEmail("user@example.com"), gomock.Any()).
		Return(secureoperation.SecureOperation{}, nil)
	s.operationResponse.EXPECT().
		NewConfirmOperation(gomock.Any(), "confirm it").
		Return(model.WaitingConfirmOperationResponse{})

	s.Require().NoError(s.signup())
}

// TestSignupThrottledSetsRetryAfter - анти-спам троттл повторной регистрации отдаётся
// приложением как 429, а 429 без Retry-After не сообщает клиенту ничего: он вынужден
// подбирать паузу сам. Срок приносит сама ошибка (окно троттла знает usecase, а не
// контроллер); ошибка без срока - отдельный случай: называть нечего, и выдумывать
// число вместо честного отсутствия заголовка нельзя.
func (s *AuthSuite) TestSignupThrottledSetsRetryAfter() {
	tests := []struct {
		name           string
		err            error
		wantRetryAfter string
	}{
		{
			name:           "throttle window is known",
			err:            mrauth.ErrSignupAlreadyInProgressTryLater.Wrap(mrauth.NewRetryAfterError(90 * time.Second)),
			wantRetryAfter: "90",
		},
		{
			// дробная длительность округляется вверх: по значению вниз клиент пришёл бы раньше срока
			name:           "fractional second is rounded up",
			err:            mrauth.ErrSignupAlreadyInProgressTryLater.Wrap(mrauth.NewRetryAfterError(1500 * time.Millisecond)),
			wantRetryAfter: "2",
		},
		{
			name:           "window is not configured",
			err:            mrauth.ErrSignupAlreadyInProgressTryLater.Wrap(mrauth.NewRetryAfterError(0)),
			wantRetryAfter: "",
		},
		{
			// сентинел без срока: заголовка нет, но ошибка обязана дойти до клиента как есть
			name:           "error does not carry the window",
			err:            mrauth.ErrSignupAlreadyInProgressTryLater,
			wantRetryAfter: "",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.expectRequestSettings("en-US", "Asia/Tokyo")
			expectValidate(s, model.CreateUserRequest{Realm: "shop", UserEmail: "user@example.com"})
			s.parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{})
			s.useCaseCreateUser.EXPECT().
				Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(secureoperation.SecureOperation{}, tt.err)

			err := s.signup()
			s.Require().ErrorIs(err, mrauth.ErrSignupAlreadyInProgressTryLater)
			s.Equal(tt.wantRetryAfter, s.rec.Header().Get("Retry-After"))
		})
	}
}

// TestSignupEmailAlreadyExistsWithoutRetryAfter - постоянный отказ (емаил занят) остаётся
// привязанной к полю ошибкой 400 и заголовка повторной попытки не получает: Retry-After
// выставляется только там, где отказ временный.
func (s *AuthSuite) TestSignupEmailAlreadyExistsWithoutRetryAfter() {
	s.expectRequestSettings("en-US", "Asia/Tokyo")
	expectValidate(s, model.CreateUserRequest{Realm: "shop", UserEmail: "user@example.com"})
	s.parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{})
	s.useCaseCreateUser.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(secureoperation.SecureOperation{}, mrauth.ErrEmailAlreadyExists)

	err := s.signup()
	s.Require().ErrorIs(err, mrauth.ErrEmailAlreadyExists)
	s.Empty(s.rec.Header().Get("Retry-After"))
}

// TestOpenSessionWithoutSecret - секрет указывается, только пока в операции остаётся
// неподтверждённое звено; по уже подтверждённой операции он не отправляется. Отсутствующее поле
// обязано доехать до подтверждения пустой строкой, а не быть отклонённым контроллером, иначе
// вход после `204` на подтверждении и повтор входа после сбоя завершить было бы нечем.
func (s *AuthSuite) TestOpenSessionWithoutSecret() {
	confirmedOp := secureoperation.SecureOperation{Status: operationstatus.Confirmed}

	s.expectRequestSettings("en-US", "Asia/Tokyo")
	expectValidate(s, model.LoginByTokenRequest{Token: "op-token", Secret: nil})
	s.parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{}).AnyTimes()
	s.useCaseConfirmOp.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), "op-token", "").
		Return(confirmedOp, nil)
	s.useCaseOpenSession.EXPECT().
		Execute(gomock.Any(), gomock.Any(), confirmedOp).
		Return(
			dto.AuthTokenPair{
				Access:  dto.AccessToken{Token: "access-token", ExpiresIn: time.Hour},
				Refresh: dto.RefreshToken{Token: "refresh-token"},
			},
			nil,
		)
	s.sender.EXPECT().
		Send(gomock.Any(), http.StatusCreated, gomock.Any()).
		DoAndReturn(func(_ http.ResponseWriter, _ int, structure any) error {
			s.sent = structure

			return nil
		})

	s.Require().NoError(s.openSession())
	s.Equal(
		model.SuccessAccessResponse{
			AccessToken:  "access-token",
			ExpiresIn:    3600,
			RefreshToken: "refresh-token",
		},
		s.sent,
	)
}

// TestOpenSessionWithoutSecretOnOpenedOperation - секрет не передан, а звено операции ещё
// открыто: подтверждать нечем. Клиент обязан получить именно ConfirmCodeIsRequired, привязанный
// к полю secret, вместе с состоянием операции - иначе он не отличит «код не введён» от «код
// неверен» и покажет ошибку ввода на незаполненном поле. Ответ отправляется прямо из ветки
// подтверждения, поэтому сессия не открывается, а из обработчика ошибка не возвращается.
func (s *AuthSuite) TestOpenSessionWithoutSecretOnOpenedOperation() {
	openedOp := secureoperation.SecureOperation{Token: "op-token", RemainingAttempts: 3}
	wantResponse := model.ErrorConfirmOperationResponse{
		OperationState: model.ConfirmOperationState{RemainingAttempts: 3},
	}

	s.expectRequestSettings("en-US", "Asia/Tokyo")
	expectValidate(s, model.LoginByTokenRequest{Token: "op-token", Secret: nil})
	s.parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{}).AnyTimes()
	s.localizer.EXPECT().TranslateError(gomock.Any()).Return("confirm code is required")
	s.useCaseConfirmOp.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), "op-token", "").
		Return(openedOp, secureoperation.ErrConfirmCodeIsRequired)
	s.operationResponse.EXPECT().
		NewErrorConfirmOperation(gomock.Any(), openedOp).
		DoAndReturn(func(response mrresp.Error400Response, _ secureoperation.SecureOperation) model.ErrorConfirmOperationResponse {
			s.Require().Len(response.Errors, 1)
			s.Equal("ConfirmCodeIsRequired/secret", response.Errors[0].Code)

			return wantResponse
		})
	s.useCaseOpenSession.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.sender.EXPECT().
		Send(gomock.Any(), http.StatusBadRequest, gomock.Any()).
		DoAndReturn(func(_ http.ResponseWriter, _ int, structure any) error {
			s.sent = structure

			return nil
		})

	s.Require().NoError(s.openSession())
	s.Equal(wantResponse, s.sent)
}

// TestOpenSessionLimitExceededSetsRetryAfter - вход отклонён по hard-порогу лимита сессий:
// ошибка обязана дойти до клиента как есть (приложение отдаёт её как 429) и принести срок
// повторной попытки. Отдельно проверяется, что она не проходит через wrapOperationError:
// иначе временный отказ выглядел бы как недействительный токен операции.
func (s *AuthSuite) TestOpenSessionLimitExceededSetsRetryAfter() {
	s.sessionLimitRetryAfter = 5 * time.Minute

	confirmedOp := secureoperation.SecureOperation{Status: operationstatus.Confirmed}

	s.expectRequestSettings("en-US", "Asia/Tokyo")
	expectValidate(s, model.LoginByTokenRequest{Token: "op-token", Secret: ptr("123456")})
	s.parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{}).AnyTimes()
	s.useCaseConfirmOp.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), "op-token", "123456").
		Return(confirmedOp, nil)
	s.useCaseOpenSession.EXPECT().
		Execute(gomock.Any(), gomock.Any(), confirmedOp).
		Return(dto.AuthTokenPair{}, mrauth.ErrSessionLimitExceededTryLater)

	err := s.openSession()
	s.Require().ErrorIs(err, mrauth.ErrSessionLimitExceededTryLater)
	s.Require().NotErrorIs(err, secureoperation.ErrOperationInvalid)
	s.Equal("300", s.rec.Header().Get("Retry-After"))
}

// TestSignin - логин доезжает до usecase уже нормализованным: разделители отброшены,
// российский код 8 приведён к 7. Иначе дальше по потоку уехала бы сырая строка, а поиск
// пользователя по телефону идёт по числовому представлению логина.
func (s *AuthSuite) TestSignin() {
	expectValidate(s, model.AuthorizeUserRequest{Realm: "shop", UserLogin: "8 (999) 123-45-67"})

	s.localizer.EXPECT().Language().Return("en-US")
	s.localizer.EXPECT().Translate(gomock.Any()).Return("confirm it")
	s.parser.EXPECT().Localizer(gomock.Any()).Return(s.localizer)
	s.parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{})
	s.useCaseAuthUser.EXPECT().
		Execute(gomock.Any(), gomock.Any(), "shop", "en-US", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ dto.ActorMeta, _, _ string, userLogin contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
			s.True(userLogin.Is(addresstype.Phone))
			s.Equal("79991234567", userLogin.Value())
			s.Equal(uint64(79991234567), userLogin.DigitValue())

			return secureoperation.SecureOperation{}, nil
		})
	s.operationResponse.EXPECT().
		NewConfirmOperation(gomock.Any(), "confirm it").
		Return(model.WaitingConfirmOperationResponse{})

	s.Require().NoError(s.signin())
}

// TestUserInfo - ответ сверяется целиком, а не по отдельным полям: иначе поле, переставшее
// доезжать до клиента, осталось бы незамеченным. Разом проверяются перевод дат в пояс запроса
// (его отдаёт parser.Location), формат телефона и имя realm'а, подставленное из реестра.
func (s *AuthSuite) TestUserInfo() {
	tests := []struct {
		name          string
		registryName  string
		registryFound bool
		wantRealmName string
	}{
		{
			name:          "realm is known",
			registryName:  "site/admin",
			registryFound: true,
			wantRealmName: "site/admin",
		},
		{
			// realm'а нет в реестре: имя собирается из его идентификатора, иначе клиент
			// получил бы пустое поле вместо хоть какой-то ссылки на realm
			name:          "realm is unknown",
			registryFound: false,
			wantRealmName: "id:7",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			userID := uuid.New()
			info := okUserInfo()

			s.parser.EXPECT().UserID(gomock.Any()).Return(userID)
			s.parser.EXPECT().Location(gomock.Any()).Return(s.mustLoadLocation(responseTimeZone))
			s.realmRegistry.EXPECT().NameByID(uint16(7)).Return(tt.registryName, tt.registryFound)
			s.serviceUserInfo.EXPECT().Get(gomock.Any(), userID).Return(info, nil)

			s.Require().NoError(s.userInfo())
			s.Equal(wantUserInfoResponse(info, tt.wantRealmName), s.sent)
		})
	}
}

// TestUserInfoRecoveryCodesLeft - остаток аварийных кодов отдаётся только при включённой 2FA.
// Ноль при этом - значение, а не отсутствие: клиент по нему показывает, что коды исчерпаны,
// поэтому нулевой остаток обязан доезжать, а не опускаться вместе с выключенной 2FA.
func (s *AuthSuite) TestUserInfoRecoveryCodesLeft() {
	tests := []struct {
		name          string
		auth2FAType   auth2fatype.Enum
		recoveryCodes []string
		want          *int
	}{
		{
			name:          "2fa enabled with codes left",
			auth2FAType:   auth2fatype.TOTP,
			recoveryCodes: []string{"code-1", "code-2", "code-3"},
			want:          ptr(3),
		},
		{
			name:          "2fa enabled without codes left",
			auth2FAType:   auth2fatype.Password,
			recoveryCodes: nil,
			want:          ptr(0),
		},
		{
			// 2FA выключена: аварийных кодов нет как явления, поле не отдаётся вовсе
			name:        "2fa disabled",
			auth2FAType: auth2fatype.None,
			want:        nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			userID := uuid.New()
			info := okUserInfo()
			info.Auth2FA.Type = tt.auth2FAType
			info.Auth2FA.RecoveryCodes = tt.recoveryCodes

			s.parser.EXPECT().UserID(gomock.Any()).Return(userID)
			s.parser.EXPECT().Location(gomock.Any()).Return(s.mustLoadLocation(responseTimeZone))
			s.realmRegistry.EXPECT().NameByID(uint16(7)).Return("site/admin", true)
			s.serviceUserInfo.EXPECT().Get(gomock.Any(), userID).Return(info, nil)

			s.Require().NoError(s.userInfo())

			response, ok := s.sent.(model.UserInfoResponse)
			s.Require().True(ok)
			s.Equal(tt.want, response.RecoveryCodesLeft)
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

	err := s.newController().ChangeSettings(s.rec, r)

	s.requireRequestNotModified(r)

	return err
}

// маршрут регистрации зарегистрирован как PermissionGuestOnly, поэтому внутренние заголовки
// до обработчика не доходят: middleware отвергает запрос с access-токеном и удаляет заголовки,
// присланные клиентом самостоятельно.
func (s *AuthSuite) signup() error {
	return s.newController().Signup(
		s.rec,
		httptest.NewRequest(http.MethodPost, "/v1/signup", http.NoBody),
	)
}

func (s *AuthSuite) signin() error {
	return s.newController().Signin(
		s.rec,
		httptest.NewRequest(http.MethodPost, "/v1/signin", http.NoBody),
	)
}

// TestContinueSessionErrorIsNotFieldBound - ошибка refresh токена обязана уйти клиенту как есть,
// без привязки к полю запроса: WithCustomCode форсирует ответ 400 безусловно, а негодному
// сессионному токену положен 401.
func (s *AuthSuite) TestContinueSessionErrorIsNotFieldBound() {
	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "токен неизвестен или истёк", err: mrauth.ErrTokenNotFoundOrExpired},
		{name: "токен не разобрался", err: mrauth.ErrTokenInvalid},
	} {
		s.Run(tt.name, func() {
			s.refreshCookie.EXPECT().GetValue(gomock.Any()).Return("")
			expectValidate(s, model.ContinueSessionRequest{RefreshToken: "rt"})
			s.parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{}).AnyTimes()
			s.parser.EXPECT().Localizer(gomock.Any()).Return(s.localizer).AnyTimes()
			s.localizer.EXPECT().Language().Return("en-US").AnyTimes()
			s.useCaseContinueSess.EXPECT().
				Execute(gomock.Any(), gomock.Any(), gomock.Any(), "rt").
				Return(dto.AuthTokenPair{}, tt.err)

			err := s.newController().ContinueSession(
				s.rec,
				httptest.NewRequest(http.MethodPatch, "/v1/session", http.NoBody),
			)

			s.Require().ErrorIs(err, tt.err)

			var customErr errors.CustomError

			s.Require().NotErrorAs(
				err,
				&customErr,
				"ошибка сессионного токена не должна быть привязана к полю: иначе ответ станет 400 вместо 401",
			)
		})
	}
}

func (s *AuthSuite) openSession() error {
	return s.newController().OpenSession(
		s.rec,
		httptest.NewRequest(http.MethodPost, "/v1/session", http.NoBody),
	)
}

func (s *AuthSuite) userInfo() error {
	return s.newController().UserInfo(
		s.rec,
		httptest.NewRequest(http.MethodGet, "/v1/user", http.NoBody),
	)
}
