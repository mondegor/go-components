package unit

import (
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// NewPendingOperation - строит проекцию операции для списка личного кабинета. ok == false -
// операция в список не входит (белый список - pendingOperationType): в список попадают только
// операции, в которых может быть применён аварийный код, и долгоживущие операции; вход в аккаунт
// не отдаётся, токен чужой попытки входа нельзя показывать другой сессии.
// Нечитаемый payload - нарушение инварианта, а не повод пропустить операцию: возвращается ошибка.
func NewPendingOperation(op secureoperation.SecureOperation) (item dto.PendingOperation, ok bool, err error) {
	opType, ok := pendingOperationType(op.Name)
	if !ok {
		return dto.PendingOperation{}, false, nil
	}

	item = dto.PendingOperation{
		Token:         op.Token,
		Type:          opType,
		Status:        op.Status,
		ExpiresAt:     op.ExpiresAt,
		CurrentAction: pendingAction(&op),
	}

	if opType == operationtype.ChangeEmail || opType == operationtype.ChangeEmailConfirm {
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

// PendingOperationNames - возвращает имена операций, входящих в список личного кабинета, для
// отбора в хранилище. Должен совпадать с набором имён, известных pendingOperationType.
func PendingOperationNames() []string {
	return []string{
		NameConfirmChangeEmailRequest,
		NameConfirmChangeEmail,
		NameConfirmDisable2FA,
	}
}

// pendingOperationType - сопоставляет имя операции типу, отдаваемому клиенту; false - операция
// в список личного кабинета не входит. Набор имён должен совпадать с PendingOperationNames.
func pendingOperationType(operationName string) (operationtype.Enum, bool) {
	switch operationName {
	case NameConfirmChangeEmailRequest:
		return operationtype.ChangeEmail, true
	case NameConfirmChangeEmail:
		return operationtype.ChangeEmailConfirm, true
	case NameConfirmDisable2FA:
		return operationtype.Disable2FA, true
	default:
		return 0, false
	}
}
