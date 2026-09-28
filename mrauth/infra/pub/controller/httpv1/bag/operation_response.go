package bag

import (
	"github.com/mondegor/go-core/util/xtime"
	"github.com/mondegor/go-webcore/mrserver/mrresp"

	"github.com/mondegor/go-components/mrauth/dto"
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

	return model.WaitingConfirmOperationResponse{
		ConfirmOperationState: ro.operationState(&operation, action),
		Token:                 operation.Token,
		ConfirmMethod:         action.Method,
		Message:               message,
	}
}

// NewErrorConfirmOperation - формирует ответ об ошибке подтверждения операции.
func (ro *OperationResponse) NewErrorConfirmOperation(
	response mrresp.Error400Response,
	operation secureoperation.SecureOperation,
) model.ErrorConfirmOperationResponse {
	action, _ := operation.FirstAction()

	return model.ErrorConfirmOperationResponse{
		Error400Response: response,
		OperationState:   ro.operationState(&operation, action),
	}
}

// NewPendingOperation - формирует элемент списка действующих операций пользователя.
// expiresAt - срок действия, уже отформатированный в часовом поясе пользователя.
func (ro *OperationResponse) NewPendingOperation(item dto.PendingOperation, expiresAt string) model.PendingOperation {
	response := model.PendingOperation{
		Token:      item.Token,
		Type:       item.Type,
		ExtraValue: item.NewEmail,
		ExpiresAt:  expiresAt,
		Status:     item.Status,
	}

	action := item.CurrentAction
	if action == nil {
		return response
	}

	remainingAttempts := action.RemainingAttempts

	response.ConfirmMethod = action.Method
	response.RemainingAttempts = &remainingAttempts
	response.RemainingResends = action.RemainingResends

	if action.ResendsAt != nil {
		resendsIn := xtime.TimeLeftInSec(*action.ResendsAt)
		response.ResendsIn = &resendsIn
	}

	return response
}

// operationState - текущее состояние операции подтверждения: оставшиеся попытки,
// повторные отправки текущего действия action и время действия операции.
func (ro *OperationResponse) operationState(
	operation *secureoperation.SecureOperation,
	action secureoperation.ConfirmAction,
) model.ConfirmOperationState {
	remainingResends, resendsIn := resendsInfo(operation, action)

	return model.ConfirmOperationState{
		RemainingAttempts: operation.RemainingAttempts,
		RemainingResends:  remainingResends,
		ResendsIn:         resendsIn,
		ExpiresIn:         xtime.TimeLeftInSec(operation.ExpiresAt),
		DebugInfo:         ro.debugFunc(*operation),
	}
}

// resendsInfo - счётчики повторных отправок кода подтверждения текущим действием операции:
// сколько их осталось и через сколько секунд можно сделать следующую. Возвращает nil, когда
// повторные отправки не применимы (пароль, TOTP и аварийный код, а также операция без действий),
// чтобы эти поля не отдавались вовсе, тогда ноль в них остаётся значением: отправки исчерпаны
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
