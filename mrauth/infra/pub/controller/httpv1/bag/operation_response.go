package bag

import (
	"github.com/mondegor/go-core/util/xtime"
	"github.com/mondegor/go-webcore/mrserver/mrresp"

	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// OperationResponse - формирователь HTTP-ответов для операций подтверждения.
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

// NewConfirmOperation - формирует ответ об ожидании подтверждения операции.
func (ro *OperationResponse) NewConfirmOperation(
	operation secureoperation.SecureOperation,
	message string,
) model.WaitingConfirmOperationResponse {
	action, _ := operation.FirstAction()
	remainingResends, resendsIn := resendsInfo(&operation, action)

	return model.WaitingConfirmOperationResponse{
		Token:             operation.Token,
		ConfirmMethod:     action.Method,
		RemainingAttempts: operation.RemainingAttempts,
		RemainingResends:  remainingResends,
		ResendsIn:         resendsIn,
		ExpiresIn:         xtime.TimeLeftInSec(operation.ExpiresAt),
		Message:           message,
		DebugInfo:         ro.debugFunc(operation),
	}
}

// NewErrorConfirmOperation - формирует ответ об ошибке подтверждения операции.
func (ro *OperationResponse) NewErrorConfirmOperation(
	response mrresp.Error400Response,
	operation secureoperation.SecureOperation,
) model.ErrorConfirmOperationResponse {
	action, _ := operation.FirstAction()
	remainingResends, resendsIn := resendsInfo(&operation, action)

	return model.ErrorConfirmOperationResponse{
		Error400Response: response,
		OperationState: model.ConfirmOperationState{
			RemainingAttempts: operation.RemainingAttempts,
			RemainingResends:  remainingResends,
			ResendsIn:         resendsIn,
			ExpiresIn:         xtime.TimeLeftInSec(operation.ExpiresAt),
			DebugInfo:         ro.debugFunc(operation),
		},
	}
}

// resendsInfo - счётчики повторных отправок кода подтверждения текущим действием операции:
// сколько их осталось и через сколько секунд можно сделать следующую. Возвращает nil, когда
// повторные отправки не применимы (пароль и TOTP, а также операция без действий), чтобы эти
// поля не отдавались вовсе, тогда ноль в них остаётся значением: отправки исчерпаны
// и отправить можно прямо сейчас соответственно.
func resendsInfo(operation *secureoperation.SecureOperation, action secureoperation.ConfirmAction) (remaining *int16, in *int64) {
	// у операции без действий метод не заполнен, поэтому отдельная проверка её пустоты не нужна
	if !action.Sendable() {
		return nil, nil
	}

	remainingResends := operation.RemainingResends
	resendsIn := xtime.TimeLeftInSec(operation.ResendsAt)

	return &remainingResends, &resendsIn
}
