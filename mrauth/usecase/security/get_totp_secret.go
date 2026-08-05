package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// GetTOTPGeneratorSecret - возвращает secret TOTP-генератора строкой вместе с
	// otpauth-ссылкой: то же, что закодировано в QR-коде, но пригодное для ручного ввода.
	GetTOTPGeneratorSecret struct {
		storageOperation operationFetcher
		totpURLBuilder   totpURLBuilder
		errorWrapper     errors.Wrapper
	}

	totpURLBuilder interface {
		OTPAuthURL(accountName, secret string) string
	}
)

// NewGetTOTPGeneratorSecret - создаёт объект GetTOTPGeneratorSecret.
func NewGetTOTPGeneratorSecret(storageOperation operationFetcher, totpURLBuilder totpURLBuilder) *GetTOTPGeneratorSecret {
	return &GetTOTPGeneratorSecret{
		storageOperation: storageOperation,
		totpURLBuilder:   totpURLBuilder,
		errorWrapper:     errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - проверяет подтверждённую операцию и возвращает secret TOTP-генератора вместе с
// otpauth-ссылкой на него. Также имеется альтернатива RenderTOTPGeneratorQR.
func (uc *GetTOTPGeneratorSecret) Execute(
	ctx context.Context,
	userID uuid.UUID,
	operationToken string,
) (dto.TOTPGeneratorSecret, error) {
	payload, err := fetchConfirmedTOTPPayload(ctx, uc.storageOperation, uc.errorWrapper, userID, operationToken)
	if err != nil {
		return dto.TOTPGeneratorSecret{}, err
	}

	return dto.TOTPGeneratorSecret{
		Secret:     payload.Secret,
		OTPAuthURI: uc.totpURLBuilder.OTPAuthURL(payload.Email, payload.Secret),
	}, nil
}
