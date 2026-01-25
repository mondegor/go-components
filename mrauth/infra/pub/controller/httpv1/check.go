package httpv1

import (
	"context"
	"net/http"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-webcore/mraccess"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/validate"
)

const (
	checkCheckLoginURL           = "/v1/check/check-login"
	checkCalcPasswordStrengthURL = "/v1/check/calc-password-strength" //nolint:gosec
	checkGeneratePasswordURL     = "/v1/check/generate-password"      //nolint:gosec
)

// Check - comment struct.
type (
	Check struct {
		parser          validate.RequestParser
		sender          mrserver.ResponseSender
		serviceLogin    loginService
		servicePassword passwordService
	}

	loginService interface {
		CheckAvailabilityRealm(ctx context.Context, realm, userLogin string) error
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
) *Check {
	return &Check{
		parser:          parser,
		sender:          sender,
		serviceLogin:    serviceLogin,
		servicePassword: servicePassword,
	}
}

// Handlers - возвращает обработчики контроллера Check.
func (ht *Check) Handlers() []mrserver.HttpHandler {
	return []mrserver.HttpHandler{
		{Method: http.MethodPost, URL: checkCheckLoginURL, Permission: mraccess.PermissionAnyUser, Func: ht.CheckLogin},
		{Method: http.MethodPost, URL: checkCalcPasswordStrengthURL, Permission: mraccess.PermissionAnyUser, Func: ht.CalcPasswordStrength},
		{Method: http.MethodPost, URL: checkGeneratePasswordURL, Permission: mraccess.PermissionAnyUser, Func: ht.GeneratePassword},
	}
}

// CheckLogin - comment method.
func (ht *Check) CheckLogin(w http.ResponseWriter, r *http.Request) error {
	request := model.CheckLoginRequest{}

	if err := ht.parser.Validate(r, &request); err != nil {
		return err
	}

	if err := ht.serviceLogin.CheckAvailabilityRealm(r.Context(), request.Realm, request.UserLogin); err != nil {
		if errors.Is(err, mrauth.ErrEmailAlreadyExists) || errors.Is(err, mrauth.ErrPhoneAlreadyExists) {
			return errors.WithCustomCode(err, "userLogin")
		}

		return err
	}

	return ht.sender.SendNoContent(w)
}

// CalcPasswordStrength - comment method.
func (ht *Check) CalcPasswordStrength(w http.ResponseWriter, r *http.Request) error {
	request := model.CalcPasswordStrengthRequest{}

	if err := ht.parser.Validate(r, &request); err != nil {
		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		model.CalcPasswordStrengthResponse{
			Strength: ht.servicePassword.CalcStrength(request.Password),
		},
	)
}

// GeneratePassword - comment method.
func (ht *Check) GeneratePassword(w http.ResponseWriter, _ *http.Request) error {
	return ht.sender.Send(
		w,
		http.StatusOK,
		model.GeneratedPasswordResponse{
			Password: ht.servicePassword.Generate(),
		},
	)
}
