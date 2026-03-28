package httpv1

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrmodel"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/validate"
)

const (
	securityEmailURL         = "/v1/security/email"
	securityPhoneURL         = "/v1/security/phone"
	securityPasswordURL      = "/v1/security/password"
	securityTOTPGeneratorURL = "/v1/security/totp"
	securityDisable2FAURL    = "/v1/security/disable2fa"
	securityApplyOperation   = "/v1/security/apply-operation"
)

type (
	// Security - comment struct.
	Security struct {
		parser                        validate.RequestParser
		sender                        mrserver.FileResponseSender
		useCaseChangeEmailProperty    changeEmailUseCase
		useCaseChangePhoneProperty    changePhoneUseCase
		useCaseChangePasswordProperty changePasswordUseCase
		useCaseChangeTOTPProperty     changeTOTPGeneratorUseCase
		useCaseDisable2FA             disable2FAUseCase
		useCaseApplyOperationTOTP     applyOperationTOTPUseCase
		useCaseApplyOperation         applyOperationUseCase
		operationResponse             confirmOperationResponse
	}

	changeEmailUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, newEmail string) (secureoperation.SecureOperation, error)
	}

	changePhoneUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, newPhone string) (secureoperation.SecureOperation, error)
	}

	changePasswordUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, newPassword string) (secureoperation.SecureOperation, error)
	}

	changeTOTPGeneratorUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID) (secureoperation.SecureOperation, error)
	}

	disable2FAUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID) (secureoperation.SecureOperation, error)
	}

	applyOperationTOTPUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, operationToken string) (totpURL mrmodel.Image, err error)
	}

	applyOperationUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, operationToken string) error
	}
)

// NewSecurity - создаёт объект Security.
func NewSecurity(
	parser validate.RequestParser,
	sender mrserver.FileResponseSender,
	useCaseChangeEmailProperty changeEmailUseCase,
	useCaseChangePhoneProperty changePhoneUseCase,
	useCaseChangePasswordProperty changePasswordUseCase,
	useCaseChangeTOTPProperty changeTOTPGeneratorUseCase,
	useCaseDisable2FA disable2FAUseCase,
	useCaseApplyOperationTOTP applyOperationTOTPUseCase,
	useCaseApplyOperation applyOperationUseCase,
	operationResponse confirmOperationResponse,
) *Security {
	return &Security{
		parser:                        parser,
		sender:                        sender,
		useCaseChangeEmailProperty:    useCaseChangeEmailProperty,
		useCaseChangePhoneProperty:    useCaseChangePhoneProperty,
		useCaseChangePasswordProperty: useCaseChangePasswordProperty,
		useCaseChangeTOTPProperty:     useCaseChangeTOTPProperty,
		useCaseDisable2FA:             useCaseDisable2FA,
		useCaseApplyOperationTOTP:     useCaseApplyOperationTOTP,
		useCaseApplyOperation:         useCaseApplyOperation,
		operationResponse:             operationResponse,
	}
}

// Handlers - возвращает обработчики контроллера Security.
func (ht *Security) Handlers() []mrserver.HttpHandler {
	return []mrserver.HttpHandler{
		{Method: http.MethodPost, URL: securityEmailURL, Func: ht.ChangeEmail},
		{Method: http.MethodPost, URL: securityPhoneURL, Func: ht.ChangePhone},
		{Method: http.MethodPost, URL: securityPasswordURL, Func: ht.ChangePassword},
		{Method: http.MethodPost, URL: securityTOTPGeneratorURL, Func: ht.ChangeTOTPGenerator},
		{Method: http.MethodPatch, URL: securityTOTPGeneratorURL, Func: ht.ApplyTOTPGenerator},
		{Method: http.MethodPost, URL: securityDisable2FAURL, Func: ht.Disable2FA},
		{Method: http.MethodPatch, URL: securityApplyOperation, Func: ht.ApplyOperation},
	}
}

// ChangeEmail - comment method.
func (ht *Security) ChangeEmail(w http.ResponseWriter, r *http.Request) error {
	req := model.ChangeEmailRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	op, err := ht.useCaseChangeEmailProperty.Execute(r.Context(), ht.parser.UserID(r), req.NewEmail)
	if err != nil {
		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(
			op,
			ht.parser.Localizer(r).Translate("Confirm your operation 'change email' by code"),
		),
	)
}

// ChangePhone - comment method.
func (ht *Security) ChangePhone(w http.ResponseWriter, r *http.Request) error {
	req := model.ChangePhoneRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	op, err := ht.useCaseChangePhoneProperty.Execute(r.Context(), ht.parser.UserID(r), req.NewPhone)
	if err != nil {
		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(
			op,
			ht.parser.Localizer(r).Translate("Confirm your operation 'change phone' by code"),
		),
	)
}

// ChangePassword - comment method.
func (ht *Security) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	req := model.ChangePasswordRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	op, err := ht.useCaseChangePasswordProperty.Execute(r.Context(), ht.parser.UserID(r), req.NewPassword)
	if err != nil {
		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(
			op,
			ht.parser.Localizer(r).Translate("Confirm your operation 'change password' by code"),
		),
	)
}

// ChangeTOTPGenerator - comment method.
func (ht *Security) ChangeTOTPGenerator(w http.ResponseWriter, r *http.Request) error {
	op, err := ht.useCaseChangeTOTPProperty.Execute(r.Context(), ht.parser.UserID(r))
	if err != nil {
		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(
			op,
			ht.parser.Localizer(r).Translate("Confirm your operation 'change TOTP generator' by code"),
		),
	)
}

// Disable2FA - comment method.
func (ht *Security) Disable2FA(w http.ResponseWriter, r *http.Request) error {
	op, err := ht.useCaseDisable2FA.Execute(r.Context(), ht.parser.UserID(r))
	if err != nil {
		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(
			op,
			ht.parser.Localizer(r).Translate("Confirm your operation 'disable 2fa' by code"),
		),
	)
}

// ApplyTOTPGenerator - comment method.
func (ht *Security) ApplyTOTPGenerator(w http.ResponseWriter, r *http.Request) error {
	req := model.ApplyOperationRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	totpImage, err := ht.useCaseApplyOperationTOTP.Execute(r.Context(), ht.parser.UserID(r), req.Token)
	if err != nil {
		return err
	}

	return ht.sender.SendFile(
		r.Context(),
		w,
		totpImage.ToFile(),
	)
}

// ApplyOperation - comment method.
func (ht *Security) ApplyOperation(w http.ResponseWriter, r *http.Request) error {
	req := model.ApplyOperationRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	if err := ht.useCaseApplyOperation.Execute(r.Context(), ht.parser.UserID(r), req.Token); err != nil {
		return err
	}

	return ht.sender.SendNoContent(w)
}
