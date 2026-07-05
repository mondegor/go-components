package httpv1

import (
	"context"
	"net/http"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mraccess"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/validate"
)

const (
	checkCheckLoginURL           = "/v1/check/check-login"
	checkCalcPasswordStrengthURL = "/v1/check/calc-password-strength" //nolint:gosec
	checkGeneratePasswordURL     = "/v1/check/generate-password"      //nolint:gosec
	checkJwksURL                 = "/.well-known/jwks.json"
)

// Check - контроллер вспомогательных проверок: доступность логина, оценка и генерация пароля, выдача JWKS.
type (
	Check struct {
		parser          validate.RequestParser
		sender          mrserver.ResponseSender
		serviceLogin    loginService
		servicePassword passwordService
		jwksJsonBody    []byte
	}

	loginService interface {
		CheckAvailabilityRealm(ctx context.Context, realm string, userLogin contactaddress.ContactAddress) error
	}

	passwordService interface {
		CalcStrength(userPassword string) (strength string)
		Generate() (strength string)
	}
)

// NewCheck - создаёт объект Check.
func NewCheck(
	parser validate.RequestParser,
	sender mrserver.ResponseSender,
	serviceLogin loginService,
	servicePassword passwordService,
	jwksJsonBody []byte,
) *Check {
	return &Check{
		parser:          parser,
		sender:          sender,
		serviceLogin:    serviceLogin,
		servicePassword: servicePassword,
		jwksJsonBody:    jwksJsonBody,
	}
}

// Handlers - возвращает обработчики контроллера Check.
func (ht *Check) Handlers() []mrserver.HttpHandler {
	handlers := []mrserver.HttpHandler{
		{Method: http.MethodPost, URL: checkCheckLoginURL, Permission: mraccess.PermissionEveryone, Func: ht.CheckLogin},
		{Method: http.MethodPost, URL: checkCalcPasswordStrengthURL, Permission: mraccess.PermissionEveryone, Func: ht.CalcPasswordStrength},
		{Method: http.MethodPost, URL: checkGeneratePasswordURL, Permission: mraccess.PermissionEveryone, Func: ht.GeneratePassword},
		{Method: http.MethodGet, URL: checkJwksURL, Permission: mraccess.PermissionEveryone, Func: ht.GetJwks},
	}

	return handlers
}

// CheckLogin - проверяет доступность логина (email/телефон) для регистрации в указанном realm.
//
// Ручка by design публична (PermissionEveryone) и раскрывает занятость логина ради UX формы
// регистрации - как и говорящие ошибки в Signin/Signup. Перебор аккаунтов через неё закрывается
// не сокрытием ответа, а rate-limit'ом (отдельная задача).
// TODO: добавить rate-limit (частота проверок доступности логина по identifier+IP).
func (ht *Check) CheckLogin(w http.ResponseWriter, r *http.Request) error {
	req := model.CheckLoginRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	parsedLogin, err := contactaddress.Parse(req.UserLogin)
	if err != nil {
		return errors.WithCustomCode(errors.ErrIncorrectInputData.New(err), "userLogin")
	}

	if err := ht.serviceLogin.CheckAvailabilityRealm(r.Context(), req.Realm, parsedLogin); err != nil {
		if errors.Is(err, mrauth.ErrEmailAlreadyExists) || errors.Is(err, mrauth.ErrPhoneAlreadyExists) {
			return errors.WithCustomCode(err, "userLogin")
		}

		return err
	}

	return ht.sender.SendNoContent(w)
}

// CalcPasswordStrength - оценивает надёжность переданного пароля.
func (ht *Check) CalcPasswordStrength(w http.ResponseWriter, r *http.Request) error {
	req := model.CalcPasswordStrengthRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		model.CalcPasswordStrengthResponse{
			Strength: ht.servicePassword.CalcStrength(req.Password),
		},
	)
}

// GeneratePassword - генерирует случайный пароль.
func (ht *Check) GeneratePassword(w http.ResponseWriter, _ *http.Request) error {
	return ht.sender.Send(
		w,
		http.StatusOK,
		model.GeneratedPasswordResponse{
			Password: ht.servicePassword.Generate(),
		},
	)
}

// GetJwks - отдаёт набор публичных JWT-ключей (JWKS, RFC 7517) для проверки
// подписи выданных сервисом access-токенов.
func (ht *Check) GetJwks(w http.ResponseWriter, _ *http.Request) error {
	if ht.jwksJsonBody == nil {
		return errors.ErrRecordNotFound
	}

	return ht.sender.SendBytes(w, http.StatusOK, ht.jwksJsonBody)
}
