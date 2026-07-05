package httpv1

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mraccess"
	modelmedia "github.com/mondegor/go-sysmess/mrmodel/media"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/validate"
)

const (
	securityEmailURL               = "/v1/security/email"
	securityPhoneURL               = "/v1/security/phone"
	securityApplyOperation         = "/v1/security/apply-operation"
	securityPasswordURL            = "/v1/security/password"
	securityApplyPasswordURL       = "/v1/security/apply-password" //nolint:gosec
	securityTOTPGeneratorURL       = "/v1/security/totp"
	securityRenderTOTPGeneratorURL = "/v1/security/totp/{token}"
	securityApplyTOTPGeneratorURL  = "/v1/security/apply-totp"
	securityRecoveryCodesURL       = "/v1/security/recovery-codes"
	securityApplyRecoveryCodesURL  = "/v1/security/apply-recovery-codes"
	securityDisable2FAURL          = "/v1/security/disable2fa"
)

type (
	// Security - HTTP-контроллер операций безопасности пользователя (2FA, смена email/телефона/пароля).
	Security struct {
		parser                        validate.RequestParser
		sender                        mrserver.FileResponseSender
		useCaseChangeEmailProperty    changeEmailUseCase
		useCaseChangePhoneProperty    changePhoneUseCase
		useCaseApplyOperation         applyOperationUseCase
		useCaseChangePasswordProperty changePasswordUseCase
		useCaseApplyPassword          applyPasswordUseCase
		useCaseChangeTOTPProperty     changeTOTPGeneratorUseCase
		useCaseRenderTOTPGeneratorQR  renderTOTPGeneratorQRUseCase
		useCaseApplyTOTPGenerator     applyTOTPGeneratorUseCase
		useCaseRegenerateRecovery     regenerateRecoveryUseCase
		useCaseApplyRecovery          applyRecoveryUseCase
		useCaseDisable2FA             disable2FAUseCase
		operationResponse             confirmOperationResponse
	}

	changeEmailUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, newEmail string) (secureoperation.SecureOperation, error)
	}

	changePhoneUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, newPhone string) (secureoperation.SecureOperation, error)
	}

	applyOperationUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, operationToken string) error
	}

	changePasswordUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, newPassword string) (secureoperation.SecureOperation, error)
	}

	applyPasswordUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, operationToken string) ([]string, error)
	}

	changeTOTPGeneratorUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID) (secureoperation.SecureOperation, error)
	}

	renderTOTPGeneratorQRUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, operationToken string) (modelmedia.Image, error)
	}

	applyTOTPGeneratorUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, operationToken, totpCode string) ([]string, error)
	}

	regenerateRecoveryUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID) (secureoperation.SecureOperation, error)
	}

	applyRecoveryUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, operationToken string) ([]string, error)
	}

	disable2FAUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID) (secureoperation.SecureOperation, error)
	}
)

// NewSecurity - создаёт объект Security.
func NewSecurity(
	parser validate.RequestParser,
	sender mrserver.FileResponseSender,
	useCaseChangeEmailProperty changeEmailUseCase,
	useCaseChangePhoneProperty changePhoneUseCase,
	useCaseApplyOperation applyOperationUseCase,
	useCaseChangePasswordProperty changePasswordUseCase,
	useCaseApplyPassword applyPasswordUseCase,
	useCaseChangeTOTPProperty changeTOTPGeneratorUseCase,
	useCaseRenderTOTPGeneratorQR renderTOTPGeneratorQRUseCase,
	useCaseApplyTOTPGenerator applyTOTPGeneratorUseCase,
	useCaseRegenerateRecovery regenerateRecoveryUseCase,
	useCaseApplyRecovery applyRecoveryUseCase,
	useCaseDisable2FA disable2FAUseCase,
	operationResponse confirmOperationResponse,
) *Security {
	return &Security{
		parser:                        parser,
		sender:                        sender,
		useCaseChangeEmailProperty:    useCaseChangeEmailProperty,
		useCaseChangePhoneProperty:    useCaseChangePhoneProperty,
		useCaseApplyOperation:         useCaseApplyOperation,
		useCaseChangePasswordProperty: useCaseChangePasswordProperty,
		useCaseApplyPassword:          useCaseApplyPassword,
		useCaseChangeTOTPProperty:     useCaseChangeTOTPProperty,
		useCaseRenderTOTPGeneratorQR:  useCaseRenderTOTPGeneratorQR,
		useCaseApplyTOTPGenerator:     useCaseApplyTOTPGenerator,
		useCaseRegenerateRecovery:     useCaseRegenerateRecovery,
		useCaseApplyRecovery:          useCaseApplyRecovery,
		useCaseDisable2FA:             useCaseDisable2FA,
		operationResponse:             operationResponse,
	}
}

// Handlers - возвращает обработчики контроллера Security.
func (ht *Security) Handlers() []mrserver.HttpHandler {
	return []mrserver.HttpHandler{
		{Method: http.MethodPost, URL: securityEmailURL, Permission: mraccess.PermissionAnyUser, Func: ht.ChangeEmail},
		{Method: http.MethodPost, URL: securityPhoneURL, Permission: mraccess.PermissionAnyUser, Func: ht.ChangePhone},
		{Method: http.MethodPost, URL: securityApplyOperation, Permission: mraccess.PermissionAnyUser, Func: ht.ApplyOperation},
		{Method: http.MethodPost, URL: securityPasswordURL, Permission: mraccess.PermissionAnyUser, Func: ht.ChangePassword},
		{Method: http.MethodPost, URL: securityApplyPasswordURL, Permission: mraccess.PermissionAnyUser, Func: ht.ApplyPassword},
		{Method: http.MethodPost, URL: securityTOTPGeneratorURL, Permission: mraccess.PermissionAnyUser, Func: ht.ChangeTOTPGenerator},
		{Method: http.MethodGet, URL: securityRenderTOTPGeneratorURL, Permission: mraccess.PermissionAnyUser, Func: ht.RenderTOTPGeneratorQR},
		{Method: http.MethodPost, URL: securityApplyTOTPGeneratorURL, Permission: mraccess.PermissionAnyUser, Func: ht.ApplyTOTPGenerator},
		{Method: http.MethodPost, URL: securityRecoveryCodesURL, Permission: mraccess.PermissionAnyUser, Func: ht.RegenerateRecoveryCodes},
		{Method: http.MethodPost, URL: securityApplyRecoveryCodesURL, Permission: mraccess.PermissionAnyUser, Func: ht.ApplyRecoveryCodes},
		{Method: http.MethodPost, URL: securityDisable2FAURL, Permission: mraccess.PermissionAnyUser, Func: ht.Disable2FA},
	}
}

// ChangeEmail - создаёт операцию на изменение email пользователя.
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

// ChangePhone - создаёт операцию на установку/изменение телефона пользователя.
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

// ApplyOperation - применяет подтверждённую пользователем операцию по её токену.
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

// ChangePassword - создаёт операцию на установку/изменение пароля пользователя (2FA).
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

// ApplyPassword - применяет подтверждённую операцию смены пароля, привязывает пароль
// как 2FA и возвращает новые одноразовые аварийные коды.
func (ht *Security) ApplyPassword(w http.ResponseWriter, r *http.Request) error {
	req := model.ApplyPasswordRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	codes, err := ht.useCaseApplyPassword.Execute(r.Context(), ht.parser.UserID(r), req.Token)
	if err != nil {
		return err
	}

	return ht.sender.Send(w, http.StatusOK, model.RecoveryCodesResponse{RecoveryCodes: codes})
}

// ChangeTOTPGenerator - создаёт операцию на установку/изменение TOTP генератора пользователя.
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

// RenderTOTPGeneratorQR - возвращает QR-код TOTP генератора, построенный из секрета подтверждённой операции.
func (ht *Security) RenderTOTPGeneratorQR(w http.ResponseWriter, r *http.Request) error {
	totpImage, err := ht.useCaseRenderTOTPGeneratorQR.Execute(r.Context(), ht.parser.UserID(r), ht.getRawToken(r))
	if err != nil {
		return err
	}

	return ht.sender.SendFile(
		r.Context(),
		w,
		totpImage.ToFile(),
	)
}

// ApplyTOTPGenerator - проверяет TOTP-код, привязывает генератор и возвращает одноразовые аварийные коды.
func (ht *Security) ApplyTOTPGenerator(w http.ResponseWriter, r *http.Request) error {
	req := model.ApplyTOTPGeneratorRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	codes, err := ht.useCaseApplyTOTPGenerator.Execute(r.Context(), ht.parser.UserID(r), req.Token, req.Code)
	if err != nil {
		return err
	}

	return ht.sender.Send(w, http.StatusOK, model.RecoveryCodesResponse{RecoveryCodes: codes})
}

// RegenerateRecoveryCodes - создаёт операцию перевыпуска аварийных кодов пользователя.
func (ht *Security) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	op, err := ht.useCaseRegenerateRecovery.Execute(r.Context(), ht.parser.UserID(r))
	if err != nil {
		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(
			op,
			ht.parser.Localizer(r).Translate("Confirm your operation 'regenerate recovery codes' by code"),
		),
	)
}

// ApplyRecoveryCodes - применяет подтверждённую операцию перевыпуска аварийных кодов
// и возвращает новый набор одноразовых кодов.
func (ht *Security) ApplyRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	req := model.ApplyRecoveryCodesRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	codes, err := ht.useCaseApplyRecovery.Execute(r.Context(), ht.parser.UserID(r), req.Token)
	if err != nil {
		return err
	}

	return ht.sender.Send(w, http.StatusOK, model.RecoveryCodesResponse{RecoveryCodes: codes})
}

// Disable2FA - создаёт операцию на отключение 2FA аутентификации пользователя.
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

func (ht *Security) getRawToken(r *http.Request) string {
	return ht.parser.PathParamString(r, "token")
}
