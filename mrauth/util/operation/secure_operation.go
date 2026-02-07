package operation

import (
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// ErrInternalOperationHasOnlyConfirmedActions - operation has only confirmed actions.
var (
	ErrInternalOperationHasOnlyConfirmedActions = errors.NewInternalProto("operation has only confirmed actions")
)

// NextConfirmingAction - comments func.
func NextConfirmingAction(operation *secureoperation.SecureOperation) (*secureoperation.ConfirmAction, error) {
	if operation == nil {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("operation is nil")
	}

	if operation.Status != operationstatus.Opened {
		return nil, errors.NewInternalError("operation status must be OPENED")
	}

	if len(operation.Actions) == 0 {
		return nil, errors.NewInternalError("operation does not contain any actions")
	}

	for i := range operation.Actions {
		if operation.Actions[i].Confirmed {
			continue
		}

		if operation.Actions[i].Method == 0 {
			return nil, errors.NewInternalError(
				"operation contains action without method",
				"index", i,
			)
		}

		return &operation.Actions[i], nil
	}

	return nil, ErrInternalOperationHasOnlyConfirmedActions.New()
}
