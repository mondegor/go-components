package unit

import (
	"encoding/json"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

// BuildChangePhonePayload - собирает payload операции смены телефона пользователя,
// предварительно проверив его инварианты.
func BuildChangePhonePayload(in dto.ChangePhoneOperation) ([]byte, error) {
	if err := validateChangePhonePayload(in); err != nil {
		return nil, err
	}

	value, err := json.Marshal(in)
	if err != nil {
		return nil, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not built", "operation_name", NameConfirmChangePhone)
	}

	return value, nil
}

// ParseChangePhonePayload - разбирает payload операции смены телефона пользователя и проверяет
// его инварианты (подробнее см. ParseCreateUserPayload).
func ParseChangePhonePayload(payload []byte) (dto.ChangePhoneOperation, error) {
	parsed := dto.ChangePhoneOperation{}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return dto.ChangePhoneOperation{}, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not parsed", "operation_name", NameConfirmChangePhone)
	}

	if err := validateChangePhonePayload(parsed); err != nil {
		return dto.ChangePhoneOperation{}, err
	}

	return parsed, nil
}

// validateChangePhonePayload - проверяет инварианты payload'а операции смены телефона пользователя.
func validateChangePhonePayload(in dto.ChangePhoneOperation) error {
	if in.NewPhone == 0 {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: newPhone is empty", "operation_name", NameConfirmChangePhone)
	}

	if in.Email == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: email is empty", "operation_name", NameConfirmChangePhone)
	}

	return nil
}
