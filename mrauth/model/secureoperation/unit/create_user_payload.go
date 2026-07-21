package unit

import (
	"encoding/json"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

// BuildCreateUserPayload - собирает payload операции создания пользователя,
// предварительно проверив его инварианты.
func BuildCreateUserPayload(in dto.CreateUserOperation) ([]byte, error) {
	if err := validateCreateUserPayload(in); err != nil {
		return nil, err
	}

	value, err := json.Marshal(in)
	if err != nil {
		return nil, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not built", "operation_name", NameConfirmCreateUser)
	}

	return value, nil
}

// ParseCreateUserPayload - разбирает payload операции создания пользователя и проверяет его
// инварианты. Инварианты проверяются именно на чтении, а не только на записи: между созданием
// операции и её подтверждением проходит время, payload всё это время лежит в БД как непрозрачные
// байты (bytea), которые СУБД никак не валидирует. Поэтому его целостность подтверждается на
// каждом чтении, а не предполагается по факту того, что запись прошла через BuildCreateUserPayload.
func ParseCreateUserPayload(payload []byte) (dto.CreateUserOperation, error) {
	parsed := dto.CreateUserOperation{}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return dto.CreateUserOperation{}, errors.ErrInternalIncorrectInputData.
			WithError(err, "payload is not parsed", "operation_name", NameConfirmCreateUser)
	}

	if err := validateCreateUserPayload(parsed); err != nil {
		return dto.CreateUserOperation{}, err
	}

	return parsed, nil
}

// validateCreateUserPayload - проверяет инварианты payload'а операции создания пользователя.
// UserKind и RegisteredIP намеренно не проверяются: пустой UserKind легитимен для установки
// с единственным видом пользователей, а для IP действует инвариант "RemoteAddr всегда валиден".
//
// TimeZone проверяется наравне с LangCode: пустое значение по умолчанию нигде не подставляется,
// поэтому иначе оно дошло бы до колонки users.user_timezone, и первая же выдача токена упала бы
// внутренней ошибкой уже после успешной регистрации (см. jwt.TokenIssuer.CreateTokenPair).
func validateCreateUserPayload(in dto.CreateUserOperation) error {
	if in.Realm == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: realm is empty", "operation_name", NameConfirmCreateUser)
	}

	if in.LangCode == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: langCode is empty", "operation_name", NameConfirmCreateUser)
	}

	if in.TimeZone == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: timeZone is empty", "operation_name", NameConfirmCreateUser)
	}

	if in.Email == "" {
		return errors.ErrInternalIncorrectInputData.
			WithDetails("payload: email is empty", "operation_name", NameConfirmCreateUser)
	}

	return nil
}
