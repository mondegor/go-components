package unit

import (
	"encoding/json"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

// BuildChangePasswordPayload - собирает payload операции смены пароля пользователя,
// предварительно проверив его инварианты.
func BuildChangePasswordPayload(in dto.ChangePasswordOperation) ([]byte, error) {
	if err := validateChangePasswordPayload(in); err != nil {
		return nil, err
	}

	// пароль намеренно сериализуется в payload операции - он уже захеширован при её создании
	value, err := json.Marshal(in)
	if err != nil {
		return nil, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not built", "operation_name", NameConfirmChangePassword)
	}

	return value, nil
}

// ParseChangePasswordPayload - разбирает payload операции смены пароля пользователя и проверяет
// его инварианты (подробнее см. ParseCreateUserPayload).
func ParseChangePasswordPayload(payload []byte) (dto.ChangePasswordOperation, error) {
	parsed := dto.ChangePasswordOperation{}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return dto.ChangePasswordOperation{}, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not parsed", "operation_name", NameConfirmChangePassword)
	}

	if err := validateChangePasswordPayload(parsed); err != nil {
		return dto.ChangePasswordOperation{}, err
	}

	return parsed, nil
}

// validateChangePasswordPayload - проверяет инварианты payload'а операции смены пароля пользователя.
func validateChangePasswordPayload(in dto.ChangePasswordOperation) error {
	if in.NewPassword == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: newPassword is empty", "operation_name", NameConfirmChangePassword)
	}

	if in.Email == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: email is empty", "operation_name", NameConfirmChangePassword)
	}

	return nil
}
