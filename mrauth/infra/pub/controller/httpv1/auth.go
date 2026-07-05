package httpv1

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mraccess"
	"github.com/mondegor/go-sysmess/mrtype"
	"github.com/mondegor/go-sysmess/util/casttype"
	"github.com/mondegor/go-webcore/mrserver"
	"github.com/mondegor/go-webcore/mrserver/mrresp"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/validate"
)

const (
	authSignupURL  = "/v1/signup"
	authSigninURL  = "/v1/signin"
	authSessionURL = "/v1/session"
	authUserURL    = "/v1/user"
)

type (
	// Auth - контроллер аутентификации: регистрация, вход, жизненный цикл сессии и информация о пользователе.
	Auth struct {
		parser                 validate.RequestParser
		sender                 mrserver.ResponseSender
		refreshTokenCookie     cookieValueService
		confirmFlow            confirmOperationFlow
		useCaseCreateUser      createUserUseCase
		useCaseAuthUser        authUserUseCase
		useCaseOpenSession     openSessionUseCase
		useCaseContinueSession continueSessionUseCase
		useCaseCloseSession    closeSessionUseCase
		serviceUserInfo        userInfoService
		realmRegistry          mrauth.RealmRegistry
		operationResponse      confirmOperationResponse
		debugFunc              func(value any) string
	}

	cookieValueService interface {
		GetValue(r *http.Request) string
		SetValue(w http.ResponseWriter, value string)
		RemoveValue(w http.ResponseWriter)
	}

	createUserUseCase interface {
		Execute(
			ctx context.Context,
			realm, langCode, userEmail string,
			registeredIP mrtype.DetailedIP,
		) (secureoperation.SecureOperation, error)
	}

	authUserUseCase interface {
		Execute(ctx context.Context, realm, langCode, userLogin string) (secureoperation.SecureOperation, error)
	}

	confirmOperationUseCase interface {
		Execute(ctx context.Context, langCode, operationToken, confirmCode string) (secureoperation.SecureOperation, error)
	}

	openSessionUseCase interface {
		Execute(ctx context.Context, meta dto.SessionMeta, op secureoperation.SecureOperation) (token dto.AuthTokenPair, err error)
	}

	continueSessionUseCase interface {
		Execute(ctx context.Context, langCode, refreshToken string) (token dto.AuthTokenPair, err error)
	}

	closeSessionUseCase interface {
		Execute(ctx context.Context, refreshToken string) error
	}

	userInfoService interface {
		Get(ctx context.Context, userID uuid.UUID) (dto.UserInfo, error)
	}

	confirmOperationResponse interface {
		NewConfirmOperation(operation secureoperation.SecureOperation, message string) model.WaitingConfirmOperationResponse
		NewErrorConfirmOperation(response mrresp.Error400Response, operation secureoperation.SecureOperation) model.ErrorConfirmOperationResponse
	}
)

// NewAuth - создаёт контроллер Auth.
func NewAuth(
	parser validate.RequestParser,
	sender mrserver.ResponseSender,
	refreshTokenCookie cookieValueService,
	useCaseCreateUser createUserUseCase,
	useCaseConfirmAuthUser authUserUseCase,
	useCaseConfirmOperation confirmOperationUseCase,
	useCaseOpenSession openSessionUseCase,
	useCaseContinueSession continueSessionUseCase,
	useCaseCloseSession closeSessionUseCase,
	serviceUserInfo userInfoService,
	realmRegistry mrauth.RealmRegistry,
	operationResponse confirmOperationResponse,
	debugFunc func(value any) string,
) *Auth {
	if debugFunc == nil {
		debugFunc = func(_ any) string {
			return ""
		}
	}

	return &Auth{
		parser:             parser,
		sender:             sender,
		refreshTokenCookie: refreshTokenCookie,
		confirmFlow: confirmOperationFlow{
			parser:            parser,
			sender:            sender,
			useCase:           useCaseConfirmOperation,
			operationResponse: operationResponse,
			debugFunc:         debugFunc,
		},
		useCaseCreateUser:      useCaseCreateUser,
		useCaseAuthUser:        useCaseConfirmAuthUser,
		useCaseOpenSession:     useCaseOpenSession,
		useCaseContinueSession: useCaseContinueSession,
		useCaseCloseSession:    useCaseCloseSession,
		serviceUserInfo:        serviceUserInfo,
		realmRegistry:          realmRegistry,
		operationResponse:      operationResponse,
		debugFunc:              debugFunc,
	}
}

// Handlers - возвращает обработчики контроллера Auth.
func (ht *Auth) Handlers() []mrserver.HttpHandler {
	return []mrserver.HttpHandler{
		{Method: http.MethodPost, URL: authSignupURL, Permission: mraccess.PermissionGuestOnly, Func: ht.Signup},
		{Method: http.MethodPost, URL: authSigninURL, Permission: mraccess.PermissionGuestOnly, Func: ht.Signin},
		{Method: http.MethodPost, URL: authSessionURL, Permission: mraccess.PermissionGuestOnly, Func: ht.OpenSession},
		{Method: http.MethodPatch, URL: authSessionURL, Permission: mraccess.PermissionEveryone, Func: ht.ContinueSession},
		{Method: http.MethodDelete, URL: authSessionURL, Permission: mraccess.PermissionAnyUser, Func: ht.CloseSession},
		{Method: http.MethodGet, URL: authUserURL, Permission: mraccess.PermissionAnyUser, Func: ht.UserInfo},
	}
}

// Signup - принимает запрос на регистрацию пользователя и инициирует подтверждение операции по коду.
func (ht *Auth) Signup(w http.ResponseWriter, r *http.Request) error {
	req := model.CreateUserRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	lz := ht.parser.Localizer(r)

	// занятость email раскрывается осознанно (ErrEmailAlreadyExists), как в check-login и Signin -
	// это by design ради UX формы регистрации; перебор аккаунтов закрывается rate-limit'ом (отдельная задача).
	// TODO: добавить rate-limit (частота регистраций/повторной отправки кода по identifier+IP)
	op, err := ht.useCaseCreateUser.Execute(r.Context(), req.Realm, lz.Language(), req.UserEmail, ht.parser.DetailedIP(r))
	if err != nil {
		if errors.Is(err, mrauth.ErrEmailAlreadyExists) {
			return errors.WithCustomCode(err, "userEmail")
		}

		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(
			op,
			lz.Translate("Confirm the creation of the user by code"),
		),
	)
}

// Signin - принимает запрос на вход пользователя и инициирует подтверждение операции по коду.
func (ht *Auth) Signin(w http.ResponseWriter, r *http.Request) error {
	req := model.AuthorizeUserRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	lz := ht.parser.Localizer(r)

	// существование логина раскрывается осознанно (ErrLoginNotExists), как в check-login и Signup -
	// это by design ради UX формы входа; перебор аккаунтов закрывается rate-limit'ом (отдельная задача).
	// TODO: добавить rate-limit (частота попыток входа/повторной отправки кода по identifier+IP)
	op, err := ht.useCaseAuthUser.Execute(r.Context(), req.Realm, lz.Language(), req.UserLogin)
	if err != nil {
		if errors.Is(err, mrauth.ErrLoginNotExists) {
			return errors.WithCustomCode(err, "userLogin")
		}

		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(
			op,
			lz.Translate("Confirm your identity to sign in by code"),
		),
	)
}

// OpenSession - открывает сессию после подтверждённой операции и возвращает пару токенов.
func (ht *Auth) OpenSession(w http.ResponseWriter, r *http.Request) error {
	req := model.LoginByTokenRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	// шаг 1: подтвердить операцию (включая ветку 2FA)
	op, ok, err := ht.confirmFlow.confirm(w, r, req.Token, req.Secret, "Confirm your identity to sign in by 2fa")
	if err != nil {
		return err // ошибка подтверждения операции
	}

	if !ok {
		return nil // требуется доп. подтверждение (2FA) или код неверен — ответ уже отправлен
	}

	// шаг 2: открыть сессию и выдать пару токенов
	tk, err := ht.useCaseOpenSession.Execute(
		r.Context(),
		dto.SessionMeta{
			UserAgent: r.UserAgent(),
			ClientIP:  ht.parser.DetailedIP(r),
		},
		op,
	)
	if err != nil {
		return err
	}

	if r.Header.Get("X-Use-Cookie") == "true" {
		// for web version
		ht.refreshTokenCookie.SetValue(w, tk.Refresh.Token)
		tk.Refresh.Token = ""
	}

	return ht.sender.Send(
		w,
		http.StatusCreated,
		model.SuccessAccessResponse{
			AccessToken:  tk.Access.Token,
			ExpiresIn:    uint32(tk.Access.ExpiresIn / time.Second), //nolint:gosec
			RefreshToken: tk.Refresh.Token,                          // empty for web version
		},
	)
}

// ContinueSession - продлевает сессию: перевыпускает пару токенов по refresh токену.
func (ht *Auth) ContinueSession(w http.ResponseWriter, r *http.Request) error {
	refreshToken := ht.refreshTokenCookie.GetValue(r)
	useCookie := true

	if refreshToken == "" {
		req := model.ContinueSessionRequest{}

		if err := ht.parser.Validate(r, &req); err != nil {
			return err
		}

		refreshToken = req.RefreshToken
		useCookie = false
	}

	tk, err := ht.useCaseContinueSession.Execute(r.Context(), ht.parser.Localizer(r).Language(), refreshToken)
	if err != nil {
		if errors.Is(err, errors.ErrRecordNotFound) {
			return mrauth.ErrTokenNotFoundOrExpired
		}

		return err
	}

	if useCookie {
		// for web version
		ht.refreshTokenCookie.SetValue(w, tk.Refresh.Token)
		tk.Refresh.Token = ""
	}

	return ht.sender.Send(
		w,
		http.StatusCreated,
		model.SuccessAccessResponse{
			AccessToken:  tk.Access.Token,
			ExpiresIn:    uint32(tk.Access.ExpiresIn / time.Second), //nolint:gosec
			RefreshToken: tk.Refresh.Token,
		},
	)
}

// CloseSession - закрывает сессию (logout) по refresh токену.
func (ht *Auth) CloseSession(w http.ResponseWriter, r *http.Request) error {
	refreshToken := ht.refreshTokenCookie.GetValue(r)
	useCookie := refreshToken != ""

	if !useCookie {
		req := model.CloseSessionRequest{}

		if err := ht.parser.Validate(r, &req); err != nil {
			return err
		}

		refreshToken = req.RefreshToken
	}

	if err := ht.useCaseCloseSession.Execute(r.Context(), refreshToken); err != nil {
		return err
	}

	if useCookie {
		// for web version
		ht.refreshTokenCookie.RemoveValue(w)
	}

	return ht.sender.SendNoContent(w)
}

// UserInfo - возвращает информацию о текущем пользователе.
func (ht *Auth) UserInfo(w http.ResponseWriter, r *http.Request) error {
	info, err := ht.serviceUserInfo.Get(r.Context(), ht.parser.UserID(r))
	if err != nil {
		return err
	}

	realms := make([]model.UserRealm, 0, len(info.Realms))
	for _, realm := range info.Realms {
		realmName, ok := ht.realmRegistry.NameByID(realm.RealmID)
		if !ok {
			realmName = "id:" + strconv.FormatUint(uint64(realm.RealmID), 10)
		}

		realms = append(
			realms,
			model.UserRealm{
				Name:      realmName,
				UserKind:  realm.Kind,
				CreatedAt: realm.CreatedAt.Round(1 * time.Second).Format(time.RFC3339),
				UpdatedAt: realm.UpdatedAt.Round(1 * time.Second).Format(time.RFC3339),
			},
		)
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		model.UserInfoResponse{
			Email:        info.User.Email,
			Phone:        casttype.UintToPhone(info.User.Phone),
			LangCode:     info.User.LangCode,
			LastLoginIP:  info.Stat.LastLoginIP.Real.String(),
			LastLoggedAt: info.Stat.LastLoggedAt.Round(1 * time.Second).Format(time.RFC3339),
			Auth2FAType:  info.Auth2FA.Type,
			Realms:       realms,
			Status:       info.User.Status,
			CreatedAt:    info.User.CreatedAt.Round(1 * time.Second).Format(time.RFC3339),
			UpdatedAt:    info.User.UpdatedAt.Round(1 * time.Second).Format(time.RFC3339),
		},
	)
}
