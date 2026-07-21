package unit

import (
	"encoding/json"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

// BuildChangeTOTPPayload - собирает payload операции смены TOTP пользователя,
// предварительно проверив его инварианты.
func BuildChangeTOTPPayload(in dto.ChangeTOTPOperation) ([]byte, error) {
	if err := validateChangeTOTPPayload(in); err != nil {
		return nil, err
	}

	value, err := json.Marshal(in) //nolint:gosec // G117: TOTP-secret намеренно сериализуется в payload операции для последующей привязки.
	if err != nil {
		return nil, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not built", "operation_name", NameConfirmChangeTOTP)
	}

	return value, nil
}

// ParseChangeTOTPPayload - разбирает payload операции смены TOTP пользователя и проверяет
// его инварианты (подробнее см. ParseCreateUserPayload).
func ParseChangeTOTPPayload(payload []byte) (dto.ChangeTOTPOperation, error) {
	parsed := dto.ChangeTOTPOperation{}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return dto.ChangeTOTPOperation{}, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not parsed", "operation_name", NameConfirmChangeTOTP)
	}

	if err := validateChangeTOTPPayload(parsed); err != nil {
		return dto.ChangeTOTPOperation{}, err
	}

	return parsed, nil
}

// validateChangeTOTPPayload - проверяет инварианты payload'а операции смены TOTP пользователя.
func validateChangeTOTPPayload(in dto.ChangeTOTPOperation) error {
	if in.Secret == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: secret is empty", "operation_name", NameConfirmChangeTOTP)
	}

	if in.Email == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: email is empty", "operation_name", NameConfirmChangeTOTP)
	}

	return nil
}
