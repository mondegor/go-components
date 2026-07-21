package unit

import (
	"encoding/json"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

// BuildRegenerateRecoveryPayload - собирает payload операции перевыпуска аварийных кодов
// пользователя, предварительно проверив его инварианты.
func BuildRegenerateRecoveryPayload(in dto.OperationWithUserEmail) ([]byte, error) {
	if err := validateRegenerateRecoveryPayload(in); err != nil {
		return nil, err
	}

	value, err := json.Marshal(in)
	if err != nil {
		return nil, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not built", "operation_name", NameConfirmRegenerateRecovery)
	}

	return value, nil
}

// ParseRegenerateRecoveryPayload - разбирает payload операции перевыпуска аварийных кодов
// пользователя и проверяет его инварианты (подробнее о том, зачем это делается на чтении,
// - в ParseCreateUserPayload).
func ParseRegenerateRecoveryPayload(payload []byte) (dto.OperationWithUserEmail, error) {
	parsed := dto.OperationWithUserEmail{}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return dto.OperationWithUserEmail{}, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not parsed", "operation_name", NameConfirmRegenerateRecovery)
	}

	if err := validateRegenerateRecoveryPayload(parsed); err != nil {
		return dto.OperationWithUserEmail{}, err
	}

	return parsed, nil
}

// validateRegenerateRecoveryPayload - проверяет инварианты payload'а операции перевыпуска
// аварийных кодов пользователя.
func validateRegenerateRecoveryPayload(in dto.OperationWithUserEmail) error {
	if in.Email == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: email is empty", "operation_name", NameConfirmRegenerateRecovery)
	}

	return nil
}
