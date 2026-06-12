package httpv1

import (
	"context"
	"net/http"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mraccess"
	"github.com/mondegor/go-webcore/mrserver"
	"github.com/mondegor/go-webcore/mrserver/mrresp"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/validate"
)

const (
	operationConfirmURL = "/v1/operation/confirm"
	operationResendURL  = "/v1/operation/resend"
	// operationRevokeURL  = "/v1/operation/revoke".
)

// Operation - comment struct.
type (
	Operation struct {
		parser                   validate.RequestParser
		sender                   mrserver.ResponseSender
		useCaseConfirmOperation  confirmOperationUseCase
		useCaseResendConfirmCode resendConfirmCodeUseCase
		operationResponse        confirmOperationResponse
		debugFunc                func(value any) string
	}

	resendConfirmCodeUseCase interface {
		Execute(ctx context.Context, langCode, operationToken string) (secureoperation.SecureOperation, error)
	}
)

// NewOperation - создаёт объект Operation.
func NewOperation(
	parser validate.RequestParser,
	sender mrserver.ResponseSender,
	useCaseConfirmOperation confirmOperationUseCase,
	useCaseResendConfirmCode resendConfirmCodeUseCase,
	operationResponse confirmOperationResponse,
	debugFunc func(value any) string,
) *Operation {
	if debugFunc == nil {
		debugFunc = func(_ any) string {
			return ""
		}
	}

	return &Operation{
		parser:                   parser,
		sender:                   sender,
		useCaseConfirmOperation:  useCaseConfirmOperation,
		useCaseResendConfirmCode: useCaseResendConfirmCode,
		operationResponse:        operationResponse,
		debugFunc:                debugFunc,
	}
}

// Handlers - возвращает обработчики контроллера Operation.
func (ht *Operation) Handlers() []mrserver.HttpHandler {
	return []mrserver.HttpHandler{
		{Method: http.MethodPatch, URL: operationConfirmURL, Permission: mraccess.PermissionEveryone, Func: ht.Confirm},
		{Method: http.MethodPatch, URL: operationResendURL, Permission: mraccess.PermissionEveryone, Func: ht.Resend},
		// {Method: http.MethodPatch, URL: operationRevokeURL, Permission: mraccess.PermissionAnyUser, Func: ht.Revoke},
	}
}

// Confirm - comment method.
func (ht *Operation) Confirm(w http.ResponseWriter, r *http.Request) error {
	req := model.ConfirmOperationRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	lz := ht.parser.Localizer(r)

	op, err := ht.useCaseConfirmOperation.Execute(r.Context(), lz.Language(), req.Token, req.Secret)
	if err != nil {
		if errors.Is(err, secureoperation.ErrConfirmCodeIsIncorrect) || errors.Is(err, secureoperation.ErrNoAttemptsToConfirmOperation) {
			return ht.sender.Send(
				w,
				http.StatusBadRequest,
				ht.operationResponse.NewErrorConfirmOperation(
					mrresp.NewError400Response(
						r,
						mrresp.ErrorAttribute{
							Code:      "secret",
							Detail:    lz.TranslateError(err),
							DebugInfo: ht.debugFunc(err),
						},
					),
					op,
				),
			)
		}

		return ht.wrapError(err)
	}

	// если необходимо дополнительное подтверждение (2fa)
	if op.Is(operationstatus.Opened) {
		return ht.sender.Send(
			w,
			http.StatusOK,
			ht.operationResponse.NewConfirmOperation(
				op,
				lz.Translate("Confirm your operation by 2fa"),
			),
		)
	}

	// если операция была подтверждена
	return ht.sender.SendNoContent(w)
}

// Resend - comment method.
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

		return ht.wrapError(err)
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

// func (ht *Operation) Revoke(w http.ResponseWriter, r *http.Request) error {
// 	req := OperationRequest{}
//
// 	if err := ht.parser.Validate(r, &req); err != nil {
// 		return err
// 	}
//
// 	if err := ht.useCase.Revoke(r.Context(), req.AuthToken); err != nil {
// 		return ht.wrapError(err)
// 	}
//
// 	return ht.sender.SendNoContent(w)
// }

func (ht *Operation) wrapError(err error) error {
	// ConfirmCode is not correct
	// operation already confirmed | operation is not opened
	return err
}
