package httpv1

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mraccess"
	modelmedia "github.com/mondegor/go-core/mrmodel/media"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/validate"
)

const (
	securityEmailURL               = "/v1/security/email"
	securityEmailRecoveryURL       = "/v1/security/email/recovery"
	securityApplyEmailURL          = "/v1/security/apply-email"
	securityPhoneURL               = "/v1/security/phone"
	securityApplyOperation         = "/v1/security/apply-operation"
	securityPasswordURL            = "/v1/security/password"
	securityApplyPasswordURL       = "/v1/security/apply-password" //nolint:gosec
	securityTOTPGeneratorURL       = "/v1/security/totp"
	securityTOTPGeneratorSecretURL = "/v1/security/totp/{token}"
	securityRenderTOTPGeneratorURL = "/v1/security/totp/{token}/qrcode"
	securityApplyTOTPGeneratorURL  = "/v1/security/apply-totp"
	securityRecoveryCodesURL       = "/v1/security/recovery-codes"
	securityApplyRecoveryCodesURL  = "/v1/security/apply-recovery-codes"
	securityDisable2FAURL          = "/v1/security/disable2fa"
)

type (
	// Security - HTTP-контроллер операций безопасности пользователя (2FA, смена email/телефона/пароля).
	Security struct {
		parser                               validate.RequestParser
		sender                               mrserver.FileResponseSender
		useCaseChangeEmailProperty           changeEmailUseCase
		useCaseChangeEmailByRecoveryProperty changeEmailUseCase
		useCaseApplyEmail                    applyEmailUseCase
		useCaseChangePhoneProperty           changePhoneUseCase
		useCaseApplyOperation                applyOperationUseCase
		useCaseChangePasswordProperty        changePasswordUseCase
		useCaseApplyPassword                 applyPasswordUseCase
		useCaseChangeTOTPProperty            changeTOTPGeneratorUseCase
		useCaseGetTOTPGeneratorSecret        getTOTPGeneratorSecretUseCase
		useCaseRenderTOTPGeneratorQR         renderTOTPGeneratorQRUseCase
		useCaseApplyTOTPGenerator            applyTOTPGeneratorUseCase
		useCaseRegenerateRecovery            regenerateRecoveryUseCase
		useCaseApplyRecovery                 applyRecoveryUseCase
		useCaseDisable2FA                    disable2FAUseCase
		operationResponse                    confirmOperationResponse
	}

	changeEmailUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta, newEmail contactaddress.ContactAddress) (secureoperation.SecureOperation, error)
	}

	applyEmailUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta, operationToken string) (secureoperation.SecureOperation, error)
	}

	changePhoneUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta, newPhone contactaddress.ContactAddress) (secureoperation.SecureOperation, error)
	}

	applyOperationUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta, operationToken string) error
	}

	changePasswordUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta, newPassword string) (secureoperation.SecureOperation, error)
	}

	applyPasswordUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta, operationToken string) ([]string, error)
	}

	changeTOTPGeneratorUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta) (secureoperation.SecureOperation, error)
	}

	getTOTPGeneratorSecretUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, operationToken string) (dto.TOTPGeneratorSecret, error)
	}

	renderTOTPGeneratorQRUseCase interface {
		Execute(ctx context.Context, userID uuid.UUID, operationToken string) (modelmedia.Image, error)
	}

	applyTOTPGeneratorUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta, operationToken, totpCode string) ([]string, error)
	}

	regenerateRecoveryUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta) (secureoperation.SecureOperation, error)
	}

	applyRecoveryUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta, operationToken string) ([]string, error)
	}

	disable2FAUseCase interface {
		Execute(ctx context.Context, actor dto.ActorMeta) (secureoperation.SecureOperation, error)
	}
)

// NewSecurity - создаёт объект Security.
func NewSecurity(
	parser validate.RequestParser,
	sender mrserver.FileResponseSender,
	useCaseChangeEmailProperty changeEmailUseCase,
	useCaseChangeEmailByRecoveryProperty changeEmailUseCase,
	useCaseApplyEmail applyEmailUseCase,
	useCaseChangePhoneProperty changePhoneUseCase,
	useCaseApplyOperation applyOperationUseCase,
	useCaseChangePasswordProperty changePasswordUseCase,
	useCaseApplyPassword applyPasswordUseCase,
	useCaseChangeTOTPProperty changeTOTPGeneratorUseCase,
	useCaseGetTOTPGeneratorSecret getTOTPGeneratorSecretUseCase,
	useCaseRenderTOTPGeneratorQR renderTOTPGeneratorQRUseCase,
	useCaseApplyTOTPGenerator applyTOTPGeneratorUseCase,
	useCaseRegenerateRecovery regenerateRecoveryUseCase,
	useCaseApplyRecovery applyRecoveryUseCase,
	useCaseDisable2FA disable2FAUseCase,
	operationResponse confirmOperationResponse,
) *Security {
	return &Security{
		parser:                               parser,
		sender:                               sender,
		useCaseChangeEmailProperty:           useCaseChangeEmailProperty,
		useCaseChangeEmailByRecoveryProperty: useCaseChangeEmailByRecoveryProperty,
		useCaseApplyEmail:                    useCaseApplyEmail,
		useCaseChangePhoneProperty:           useCaseChangePhoneProperty,
		useCaseApplyOperation:                useCaseApplyOperation,
		useCaseChangePasswordProperty:        useCaseChangePasswordProperty,
		useCaseApplyPassword:                 useCaseApplyPassword,
		useCaseChangeTOTPProperty:            useCaseChangeTOTPProperty,
		useCaseGetTOTPGeneratorSecret:        useCaseGetTOTPGeneratorSecret,
		useCaseRenderTOTPGeneratorQR:         useCaseRenderTOTPGeneratorQR,
		useCaseApplyTOTPGenerator:            useCaseApplyTOTPGenerator,
		useCaseRegenerateRecovery:            useCaseRegenerateRecovery,
		useCaseApplyRecovery:                 useCaseApplyRecovery,
		useCaseDisable2FA:                    useCaseDisable2FA,
		operationResponse:                    operationResponse,
	}
}

// Handlers - возвращает обработчики контроллера Security.
func (ht *Security) Handlers() []mrserver.HttpHandler {
	return []mrserver.HttpHandler{
		{Method: http.MethodPost, URL: securityEmailURL, Permission: mraccess.PermissionAnyUser, Func: ht.ChangeEmail},
		{Method: http.MethodPost, URL: securityEmailRecoveryURL, Permission: mraccess.PermissionAnyUser, Func: ht.ChangeEmailByRecovery},
		{Method: http.MethodPost, URL: securityApplyEmailURL, Permission: mraccess.PermissionAnyUser, Func: ht.ApplyEmail},
		{Method: http.MethodPost, URL: securityPhoneURL, Permission: mraccess.PermissionAnyUser, Func: ht.ChangePhone},
		{Method: http.MethodPost, URL: securityApplyOperation, Permission: mraccess.PermissionAnyUser, Func: ht.ApplyOperation},
		{Method: http.MethodPost, URL: securityPasswordURL, Permission: mraccess.PermissionAnyUser, Func: ht.ChangePassword},
		{Method: http.MethodPost, URL: securityApplyPasswordURL, Permission: mraccess.PermissionAnyUser, Func: ht.ApplyPassword},
		{Method: http.MethodPost, URL: securityTOTPGeneratorURL, Permission: mraccess.PermissionAnyUser, Func: ht.ChangeTOTPGenerator},
		{Method: http.MethodGet, URL: securityTOTPGeneratorSecretURL, Permission: mraccess.PermissionAnyUser, Func: ht.GetTOTPGeneratorSecret},
		{Method: http.MethodGet, URL: securityRenderTOTPGeneratorURL, Permission: mraccess.PermissionAnyUser, Func: ht.RenderTOTPGeneratorQR},
		{Method: http.MethodPost, URL: securityApplyTOTPGeneratorURL, Permission: mraccess.PermissionAnyUser, Func: ht.ApplyTOTPGenerator},
		{Method: http.MethodPost, URL: securityRecoveryCodesURL, Permission: mraccess.PermissionAnyUser, Func: ht.RegenerateRecoveryCodes},
		{Method: http.MethodPost, URL: securityApplyRecoveryCodesURL, Permission: mraccess.PermissionAnyUser, Func: ht.ApplyRecoveryCodes},
		{Method: http.MethodPost, URL: securityDisable2FAURL, Permission: mraccess.PermissionAnyUser, Func: ht.Disable2FA},
	}
}

// ChangeEmail - создаёт операцию на изменение email пользователя.
func (ht *Security) ChangeEmail(w http.ResponseWriter, r *http.Request) error {
	return ht.changeEmail(
		w,
		r,
		ht.useCaseChangeEmailProperty,
		"Confirm your operation 'change email' by code",
	)
}

// ChangeEmailByRecovery - создаёт операцию на изменение email пользователя, утратившего доступ
// к текущему адресу: подтверждается вторым фактором и аварийным кодом, письмо не отправляется.
// При выключенной 2FA операция не создаётся: доказательство у такого аккаунта одно - код
// на текущий адрес. Скрывать состояние 2FA не от кого, метод авторизованный.
func (ht *Security) ChangeEmailByRecovery(w http.ResponseWriter, r *http.Request) error {
	return ht.changeEmail(
		w,
		r,
		ht.useCaseChangeEmailByRecoveryProperty,
		"Confirm your operation 'change email' by second factor",
	)
}

// changeEmail - общий шаг создания операции смены email: маршруты отличаются только цепочкой
// подтверждения, то есть выбранным юзкейсом, и сообщением waitMessage.
func (ht *Security) changeEmail(
	w http.ResponseWriter,
	r *http.Request,
	useCase changeEmailUseCase,
	waitMessage string,
) error {
	req := model.ChangeEmailRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	op, err := useCase.Execute(r.Context(), ht.userActor(r), contactaddress.NewEmail(req.NewEmail))
	if err != nil {
		if errors.Is(err, mrauth.ErrEmailAlreadyExists) {
			return errors.WithCustomCode(err, "new_email")
		}

		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(op, ht.parser.Localizer(r).Translate(waitMessage)),
	)
}

// ApplyEmail - применяет подтверждённую операцию первого шага смены email: email пока
// не меняется, вместо этого открывается операция подтверждения владения новым адресом (код
// уходит на новый адрес) и возвращается клиенту. Занятый за время подтверждения адрес -
// ошибка без привязки к полю: в запросе передаётся только токен.
func (ht *Security) ApplyEmail(w http.ResponseWriter, r *http.Request) error {
	req := model.ApplyEmailRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	op, err := ht.useCaseApplyEmail.Execute(r.Context(), ht.userActor(r), req.Token)
	if err != nil {
		return wrapOperationError(err, "token")
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(
			op,
			ht.parser.Localizer(r).Translate("Confirm your new email by code"),
		),
	)
}

// ChangePhone - создаёт операцию на установку/изменение телефона пользователя.
func (ht *Security) ChangePhone(w http.ResponseWriter, r *http.Request) error {
	req := model.ChangePhoneRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	op, err := ht.useCaseChangePhoneProperty.Execute(r.Context(), ht.userActor(r), contactaddress.NewPhone(req.NewPhone))
	if err != nil {
		if errors.Is(err, mrauth.ErrPhoneAlreadyExists) {
			return errors.WithCustomCode(err, "new_phone")
		}

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

	if err := ht.useCaseApplyOperation.Execute(r.Context(), ht.userActor(r), req.Token); err != nil {
		return wrapOperationError(err, "token")
	}

	return ht.sender.SendNoContent(w)
}

// ChangePassword - создаёт операцию на установку/изменение пароля пользователя (2FA).
func (ht *Security) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	req := model.ChangePasswordRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	op, err := ht.useCaseChangePasswordProperty.Execute(r.Context(), ht.userActor(r), req.NewPassword)
	if err != nil {
		if errors.Is(err, mrauth.ErrPasswordIsTooWeak) {
			return errors.WithCustomCode(err, "new_password")
		}

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

	codes, err := ht.useCaseApplyPassword.Execute(r.Context(), ht.userActor(r), req.Token)
	if err != nil {
		return wrapOperationError(err, "token")
	}

	return ht.sender.Send(w, http.StatusOK, model.RecoveryCodesResponse{RecoveryCodes: codes})
}

// ChangeTOTPGenerator - создаёт операцию на установку/изменение TOTP генератора пользователя.
func (ht *Security) ChangeTOTPGenerator(w http.ResponseWriter, r *http.Request) error {
	op, err := ht.useCaseChangeTOTPProperty.Execute(r.Context(), ht.userActor(r))
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

// GetTOTPGeneratorSecret - возвращает secret TOTP генератора и otpauth-ссылку на него
// для ручного добавления генератора в приложение (альтернатива сканированию QR-кода).
func (ht *Security) GetTOTPGeneratorSecret(w http.ResponseWriter, r *http.Request) error {
	item, err := ht.useCaseGetTOTPGeneratorSecret.Execute(r.Context(), ht.parser.UserID(r), ht.getRawToken(r))
	if err != nil {
		return wrapOperationError(err, "") // токен пришёл path-параметром, поля запроса нет
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		model.TOTPGeneratorSecretResponse{
			Secret:     item.Secret,
			OTPAuthURI: item.OTPAuthURI,
		},
	)
}

// RenderTOTPGeneratorQR - возвращает QR-код TOTP генератора, построенный из секрета подтверждённой операции.
func (ht *Security) RenderTOTPGeneratorQR(w http.ResponseWriter, r *http.Request) error {
	totpImage, err := ht.useCaseRenderTOTPGeneratorQR.Execute(r.Context(), ht.parser.UserID(r), ht.getRawToken(r))
	if err != nil {
		return wrapOperationError(err, "") // токен пришёл path-параметром, поля запроса нет
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

	codes, err := ht.useCaseApplyTOTPGenerator.Execute(r.Context(), ht.userActor(r), req.Token, req.Code)
	if err != nil {
		if errors.Is(err, mrauth.ErrTOTPCodeIsIncorrect) {
			return errors.WithCustomCode(err, "totp_code")
		}

		return wrapOperationError(err, "token")
	}

	return ht.sender.Send(w, http.StatusOK, model.RecoveryCodesResponse{RecoveryCodes: codes})
}

// RegenerateRecoveryCodes - создаёт операцию перевыпуска аварийных кодов пользователя.
func (ht *Security) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	op, err := ht.useCaseRegenerateRecovery.Execute(r.Context(), ht.userActor(r))
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

	codes, err := ht.useCaseApplyRecovery.Execute(r.Context(), ht.userActor(r), req.Token)
	if err != nil {
		return wrapOperationError(err, "token")
	}

	return ht.sender.Send(w, http.StatusOK, model.RecoveryCodesResponse{RecoveryCodes: codes})
}

// Disable2FA - создаёт операцию на отключение 2FA аутентификации пользователя.
func (ht *Security) Disable2FA(w http.ResponseWriter, r *http.Request) error {
	op, err := ht.useCaseDisable2FA.Execute(r.Context(), ht.userActor(r))
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

// userActor - собирает метаданные клиента для журнала защищённых операций и уведомлений о них.
// Поток аутентифицирован (PermissionAnyUser), поэтому UserID - это сам пользователь.
func (ht *Security) userActor(r *http.Request) dto.ActorMeta {
	return dto.NewActorMeta(
		ht.parser.UserID(r),
		ht.parser.DetailedIP(r),
		r.UserAgent(),
		ht.parser.Location(r),
	)
}
