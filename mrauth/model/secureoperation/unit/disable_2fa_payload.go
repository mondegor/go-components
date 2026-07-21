package unit

import (
	"encoding/json"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

// BuildDisable2FAPayload - собирает payload операции отключения 2FA пользователя,
// предварительно проверив его инварианты.
func BuildDisable2FAPayload(in dto.Disable2FAOperation) ([]byte, error) {
	if err := validateDisable2FAPayload(in); err != nil {
		return nil, err
	}

	value, err := json.Marshal(in)
	if err != nil {
		return nil, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not built", "operation_name", NameConfirmDisable2FA)
	}

	return value, nil
}

// ParseDisable2FAPayload - разбирает payload операции отключения 2FA пользователя и проверяет
// его инварианты (подробнее см. ParseCreateUserPayload).
func ParseDisable2FAPayload(payload []byte) (dto.Disable2FAOperation, error) {
	parsed := dto.Disable2FAOperation{}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return dto.Disable2FAOperation{}, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not parsed", "operation_name", NameConfirmDisable2FA)
	}

	if err := validateDisable2FAPayload(parsed); err != nil {
		return dto.Disable2FAOperation{}, err
	}

	return parsed, nil
}

// validateDisable2FAPayload - проверяет инварианты payload'а операции отключения 2FA пользователя.
func validateDisable2FAPayload(in dto.Disable2FAOperation) error {
	if in.Email == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: email is empty", "operation_name", NameConfirmDisable2FA)
	}

	return nil
}
