package bag

import (
	"github.com/mondegor/go-sysmess/util/xtime"
	"github.com/mondegor/go-webcore/mrserver/mrresp"

	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// OperationResponse - comment struct.
type (
	OperationResponse struct {
		debugFunc func(value any) string
	}
)

// NewOperationResponse - создаёт объект OperationResponse.
func NewOperationResponse(
	debugFunc func(value any) string,
) *OperationResponse {
	if debugFunc == nil {
		debugFunc = func(_ any) string {
			return ""
		}
	}

	return &OperationResponse{
		debugFunc: debugFunc,
	}
}

// NewConfirmOperation - comment method.
func (ro *OperationResponse) NewConfirmOperation(
	operation secureoperation.SecureOperation,
	message string,
) model.WaitingConfirmOperationResponse {
	action, _ := operation.FirstAction()

	return model.WaitingConfirmOperationResponse{
		Token:             operation.Token,
		ConfirmMethod:     action.Method,
		RemainingAttempts: operation.RemainingAttempts,
		RemainingResends:  operation.RemainingResends,
		ResendsIn:         xtime.TimeLeftInSec(operation.ResendsAt),
		ExpiresIn:         xtime.TimeLeftInSec(operation.ExpiresAt),
		Message:           message,
		DebugInfo:         ro.debugFunc(operation),
	}
}

// NewErrorConfirmOperation - comment method.
func (ro *OperationResponse) NewErrorConfirmOperation(
	response mrresp.Error400Response,
	operation secureoperation.SecureOperation,
) model.ErrorConfirmOperationResponse {
	return model.ErrorConfirmOperationResponse{
		Error400Response: response,
		OperationState: model.ConfirmOperationState{
			RemainingAttempts: operation.RemainingAttempts,
			RemainingResends:  operation.RemainingResends,
			ResendsIn:         xtime.TimeLeftInSec(operation.ResendsAt),
			ExpiresIn:         xtime.TimeLeftInSec(operation.ExpiresAt),
			DebugInfo:         ro.debugFunc(operation),
		},
	}
}
