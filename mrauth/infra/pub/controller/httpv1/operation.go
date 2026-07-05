package httpv1

import (
	"context"
	"net/http"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mraccess"
	"github.com/mondegor/go-webcore/mrserver"
	"github.com/mondegor/go-webcore/mrserver/mrresp"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/validate"
)

const (
	operationConfirmURL = "/v1/operation/confirm"
	operationResendURL  = "/v1/operation/resend"
	operationRevokeURL  = "/v1/operation/revoke"
)

// Operation - HTTP-контроллер операций подтверждения (confirm/resend/revoke).
type (
	Operation struct {
		parser                   validate.RequestParser
		sender                   mrserver.ResponseSender
		confirmFlow              confirmOperationFlow
		useCaseResendConfirmCode resendConfirmCodeUseCase
		useCaseRevokeOperation   revokeOperationUseCase
		operationResponse        confirmOperationResponse
		debugFunc                func(value any) string
	}

	resendConfirmCodeUseCase interface {
		Execute(ctx context.Context, langCode, operationToken string) (secureoperation.SecureOperation, error)
	}

	revokeOperationUseCase interface {
		Execute(ctx context.Context, operationToken string) error
	}
)

// NewOperation - создаёт объект Operation.
func NewOperation(
	parser validate.RequestParser,
	sender mrserver.ResponseSender,
	useCaseConfirmOperation confirmOperationUseCase,
	useCaseResendConfirmCode resendConfirmCodeUseCase,
	useCaseRevokeOperation revokeOperationUseCase,
	operationResponse confirmOperationResponse,
	debugFunc func(value any) string,
) *Operation {
	if debugFunc == nil {
		debugFunc = func(_ any) string {
			return ""
		}
	}

	return &Operation{
		parser: parser,
		sender: sender,
		confirmFlow: confirmOperationFlow{
			parser:            parser,
			sender:            sender,
			useCase:           useCaseConfirmOperation,
			operationResponse: operationResponse,
			debugFunc:         debugFunc,
		},
		useCaseResendConfirmCode: useCaseResendConfirmCode,
		useCaseRevokeOperation:   useCaseRevokeOperation,
		operationResponse:        operationResponse,
		debugFunc:                debugFunc,
	}
}

// Handlers - возвращает обработчики контроллера Operation.
func (ht *Operation) Handlers() []mrserver.HttpHandler {
	return []mrserver.HttpHandler{
		{Method: http.MethodPatch, URL: operationConfirmURL, Permission: mraccess.PermissionEveryone, Func: ht.Confirm},
		{Method: http.MethodPatch, URL: operationResendURL, Permission: mraccess.PermissionEveryone, Func: ht.Resend},
		{Method: http.MethodPatch, URL: operationRevokeURL, Permission: mraccess.PermissionAnyUser, Func: ht.Revoke},
	}
}

// Confirm - подтверждает защищённую операцию переданным секретом.
func (ht *Operation) Confirm(w http.ResponseWriter, r *http.Request) error {
	req := model.ConfirmOperationRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	_, ok, err := ht.confirmFlow.confirm(w, r, req.Token, req.Secret, "Confirm your operation by 2fa")
	if err != nil {
		return err // ошибка подтверждения операции
	}

	if !ok {
		return nil // требуется доп. подтверждение (2FA) или код неверен — ответ уже отправлен
	}

	// если операция была подтверждена
	return ht.sender.SendNoContent(w)
}

// Resend - повторно отправляет код подтверждения операции.
func (ht *Operation) Resend(w http.ResponseWriter, r *http.Request) error {
	req := model.OperationTokenRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	lz := ht.parser.Localizer(r)

	op, err := ht.useCaseResendConfirmCode.Execute(r.Context(), lz.Language(), req.Token)
	if err != nil {
		if errors.Is(err, errors.ErrRecordNotFound) {
			return mrauth.ErrTokenNotFoundOrExpired
		}

		if errors.Is(err, secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted) {
			return ht.sender.Send(
				w,
				http.StatusBadRequest,
				ht.operationResponse.NewErrorConfirmOperation(
					mrresp.NewError400Response(
						r,
						mrresp.ErrorAttribute{
							Code:      "token",
							Detail:    lz.TranslateError(err),
							DebugInfo: ht.debugFunc(err),
						},
					),
					op,
				),
			)
		}

		return err
	}

	return ht.sender.Send(
		w,
		http.StatusOK,
		ht.operationResponse.NewConfirmOperation(
			op,
			lz.Translate("The confirmation code has been sent successfully"),
		),
	)
}

// Revoke - отзыв/отмена пользователем указанной операции.
func (ht *Operation) Revoke(w http.ResponseWriter, r *http.Request) error {
	req := model.OperationTokenRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	if err := ht.useCaseRevokeOperation.Execute(r.Context(), req.Token); err != nil {
		if errors.Is(err, errors.ErrRecordNotFound) {
			return mrauth.ErrTokenNotFoundOrExpired
		}

		return err
	}

	return ht.sender.SendNoContent(w)
}
