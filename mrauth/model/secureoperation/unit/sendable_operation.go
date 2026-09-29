package unit

import (
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// newSendableOperation - создаёт операцию, первое действие которой отправляет код, и выпускает
// этот код по длине, записанной в действие; тем же путём код выпускается при повторной отправке
// и при переходе к следующему действию.
func newSendableOperation(
	codeGenerator mrauth.CodeGenerator,
	token string,
	opType operationtype.Enum,
	userID uuid.UUID,
	actions []secureoperation.ConfirmAction,
	payload []byte,
) (secureoperation.SecureOperation, error) {
	op, err := secureoperation.NewOperation(token, opType, userID, actions, payload)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	if err = op.InitSendableAction(codeGenerator.GenCodeWithHash); err != nil {
		return secureoperation.SecureOperation{}, err
	}

	return op, nil
}
