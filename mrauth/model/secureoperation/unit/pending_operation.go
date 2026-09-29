package unit

import (
	"slices"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// NewPendingOperation - строит проекцию операции для списка личного кабинета. ok == false -
// операция в список не входит (белый список - PendingOperationTypes): в список попадают только
// операции, в которых может быть применён аварийный код, и долгоживущие операции; вход в аккаунт
// не отдаётся, токен чужой попытки входа нельзя показывать другой сессии.
// Нечитаемый payload - нарушение инварианта, а не повод пропустить операцию: возвращается ошибка.
func NewPendingOperation(op secureoperation.SecureOperation) (item dto.PendingOperation, ok bool, err error) {
	if !slices.Contains(PendingOperationTypes(), op.Type) {
		return dto.PendingOperation{}, false, nil
	}

	item = dto.PendingOperation{
		Token:         op.Token,
		Type:          op.Type,
		Status:        op.Status,
		ExpiresAt:     op.ExpiresAt,
		CurrentAction: pendingAction(&op),
	}

	if op.Type == operationtype.ChangeEmail || op.Type == operationtype.ChangeEmailConfirm {
		payload, err := ParseChangeEmailPayload(op.Payload)
		if err != nil {
			return dto.PendingOperation{}, false, err
		}

		item.NewEmail = payload.NewEmail
	}

	return item, true, nil
}

// pendingAction - текущее звено операции для показа клиенту; nil - у подтверждённой операции
// подтверждать нечего. Счётчики повторной отправки задаются, только если звено её допускает.
func pendingAction(op *secureoperation.SecureOperation) *dto.PendingAction {
	if !op.Is(operationstatus.Opened) {
		return nil
	}

	action, _ := op.FirstAction()

	pending := dto.PendingAction{
		Method:            action.Method,
		RemainingAttempts: op.RemainingAttempts,
	}

	if action.Sendable() {
		remainingResends := op.RemainingResends
		resendsAt := op.ResendsAt

		pending.RemainingResends = &remainingResends
		pending.ResendsAt = &resendsAt
	}

	return &pending
}

// PendingOperationTypes - возвращает типы операций, входящих в список личного кабинета: по ним
// отбираются операции в хранилище и фильтруются в NewPendingOperation.
func PendingOperationTypes() []operationtype.Enum {
	return []operationtype.Enum{
		operationtype.ChangeEmail,
		operationtype.ChangeEmailConfirm,
		operationtype.Disable2FA,
	}
}
