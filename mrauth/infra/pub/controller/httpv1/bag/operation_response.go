package bag

import (
	"github.com/mondegor/go-sysmess/errors/runtime/hint"
	"github.com/mondegor/go-sysmess/util/xtime"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrserver/mrresp"

	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// OperationResponse - comment struct.
type (
	OperationResponse struct {
		withDebugInfo bool
	}
)

// NewOperationResponse - создаёт объект OperationResponse.
func NewOperationResponse(withDebugInfo bool) *OperationResponse {
	return &OperationResponse{
		withDebugInfo: withDebugInfo,
	}
}

// NewConfirmOperation - comment method.
func (r *OperationResponse) NewConfirmOperation(
	operation secureoperation.SecureOperation,
	message string,
) model.WaitingConfirmOperationResponse {
	return model.WaitingConfirmOperationResponse{
		Token:             operation.Token,
		ConfirmMethod:     r.operationAction(&operation).Method,
		RemainingAttempts: operation.RemainingAttempts,
		RemainingResends:  operation.RemainingResends,
		ResendsIn:         xtime.TimeLeftInSec(operation.ResendsAt),
		ExpiresIn:         xtime.TimeLeftInSec(operation.ExpiresAt),
		Message:           message,
		DebugInfo:         r.debugInfo(&operation),
	}
}

// NewErrorConfirmOperation - comment method.
func (r *OperationResponse) NewErrorConfirmOperation(
	operation secureoperation.SecureOperation,
	lz mrcore.Localizer,
	code string,
	err error,
) model.ErrorConfirmOperationResponse {
	return model.ErrorConfirmOperationResponse{
		OperationStatus: r.newOperationStatus(&operation),
		Errors: []mrresp.ErrorAttribute{
			mrresp.NewErrorAttribute(
				code,
				lz.TranslateError(err),
				func() string { // TODO: переделать !!!!!!!!!!!!!!
					if !r.withDebugInfo {
						return ""
					}

					return hint.DetailedError(err)
				}(),
			),
		},
	}
}

func (r *OperationResponse) newOperationStatus(operation *secureoperation.SecureOperation) model.ConfirmOperationStatus {
	return model.ConfirmOperationStatus{
		RemainingAttempts: operation.RemainingAttempts,
		RemainingResends:  operation.RemainingResends,
		ResendsIn:         xtime.TimeLeftInSec(operation.ResendsAt),
		ExpiresIn:         xtime.TimeLeftInSec(operation.ExpiresAt),
		DebugInfo:         r.debugInfo(operation),
	}
}

func (r *OperationResponse) operationAction(op *secureoperation.SecureOperation) secureoperation.ConfirmAction {
	actions := op.Actions()

	if len(actions) != 0 {
		return actions[0]
	}

	return secureoperation.ConfirmAction{}
}

func (r *OperationResponse) debugInfo(op *secureoperation.SecureOperation) string {
	if !r.withDebugInfo {
		return ""
	}

	action := r.operationAction(op)

	info := "Method: " + action.Method.String()

	if action.Sendable() {
		info += ", to: " + action.Address + ", code: " + action.ConfirmCode
	}

	return info
}
