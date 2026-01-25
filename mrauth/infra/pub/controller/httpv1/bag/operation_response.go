package bag

import (
	"github.com/mondegor/go-sysmess/util/xtime"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrserver/mrresp"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
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
	operation entity.SecureOperation,
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
	operation entity.SecureOperation,
	lz mrcore.Localizer,
	code string,
	err error,
) model.ErrorConfirmOperationResponse {
	return model.ErrorConfirmOperationResponse{
		OperationStatus: r.newOperationStatus(&operation),
		Errors: []mrresp.ErrorAttribute{
			mrresp.NewErrorAttribute(lz, code, err, r.withDebugInfo),
		},
	}
}

func (r *OperationResponse) newOperationStatus(operation *entity.SecureOperation) model.ConfirmOperationStatus {
	return model.ConfirmOperationStatus{
		RemainingAttempts: operation.RemainingAttempts,
		RemainingResends:  operation.RemainingResends,
		ResendsIn:         xtime.TimeLeftInSec(operation.ResendsAt),
		ExpiresIn:         xtime.TimeLeftInSec(operation.ExpiresAt),
		DebugInfo:         r.debugInfo(operation),
	}
}

func (r *OperationResponse) operationAction(operation *entity.SecureOperation) dto.ConfirmAction {
	for i := range operation.Actions {
		if !operation.Actions[i].Confirmed {
			return operation.Actions[i]
		}
	}

	return dto.ConfirmAction{}
}

func (r *OperationResponse) debugInfo(operation *entity.SecureOperation) string {
	if !r.withDebugInfo {
		return ""
	}

	action := r.operationAction(operation)

	if action.Method == confirmmethod.Email || action.Method == confirmmethod.Phone {
		return "Confirm code: " + action.Secret
	}

	return ""
}
