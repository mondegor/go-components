package unit

import (
	"encoding/json"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

// BuildAuthorizeUserPayload - собирает payload операции авторизации пользователя,
// предварительно проверив его инварианты.
func BuildAuthorizeUserPayload(in dto.AuthorizeUserOperation) ([]byte, error) {
	if err := validateAuthorizeUserPayload(in); err != nil {
		return nil, err
	}

	value, err := json.Marshal(in)
	if err != nil {
		return nil, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not built", "operation_name", NameAuthorizeUser)
	}

	return value, nil
}

// ParseAuthorizeUserPayload - разбирает payload операции авторизации пользователя и проверяет
// его инварианты (подробнее см. ParseCreateUserPayload).
func ParseAuthorizeUserPayload(payload []byte) (dto.AuthorizeUserOperation, error) {
	parsed := dto.AuthorizeUserOperation{}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return dto.AuthorizeUserOperation{}, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not parsed", "operation_name", NameAuthorizeUser)
	}

	if err := validateAuthorizeUserPayload(parsed); err != nil {
		return dto.AuthorizeUserOperation{}, err
	}

	return parsed, nil
}

// validateAuthorizeUserPayload - проверяет инварианты payload'а операции авторизации пользователя.
func validateAuthorizeUserPayload(in dto.AuthorizeUserOperation) error {
	if in.Realm == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: realm is empty", "operation_name", NameAuthorizeUser)
	}

	if in.LangCode == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: langCode is empty", "operation_name", NameAuthorizeUser)
	}

	return nil
}
