package unit

import (
	"encoding/json"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

// BuildChangeEmailPayload - собирает payload операции смены email пользователя,
// предварительно проверив его инварианты.
func BuildChangeEmailPayload(in dto.ChangeEmailOperation) ([]byte, error) {
	if err := validateChangeEmailPayload(in); err != nil {
		return nil, err
	}

	value, err := json.Marshal(in)
	if err != nil {
		return nil, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not built", "operation_name", NameConfirmChangeEmail)
	}

	return value, nil
}

// ParseChangeEmailPayload - разбирает payload операции смены email пользователя и проверяет
// его инварианты (подробнее см. ParseCreateUserPayload).
func ParseChangeEmailPayload(payload []byte) (dto.ChangeEmailOperation, error) {
	parsed := dto.ChangeEmailOperation{}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return dto.ChangeEmailOperation{}, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not parsed", "operation_name", NameConfirmChangeEmail)
	}

	if err := validateChangeEmailPayload(parsed); err != nil {
		return dto.ChangeEmailOperation{}, err
	}

	return parsed, nil
}

// validateChangeEmailPayload - проверяет инварианты payload'а операции смены email пользователя.
func validateChangeEmailPayload(in dto.ChangeEmailOperation) error {
	if in.NewEmail == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: newEmail is empty", "operation_name", NameConfirmChangeEmail)
	}

	if in.Email == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: email is empty", "operation_name", NameConfirmChangeEmail)
	}

	return nil
}
