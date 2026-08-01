package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	operationFetcher interface {
		FetchOne(ctx context.Context, token string) (secureoperation.SecureOperation, error)
	}
)

// fetchConfirmedTOTPPayload - общий гейт методов, отдающих заготовку TOTP-генератора
// (QR-код и secret строкой): операция должна существовать, принадлежать вызывающему,
// быть именно операцией смены TOTP и быть подтверждённой. Возвращает её payload -
// емаил аккаунта и сгенерированный, ещё не привязанный secret.
func fetchConfirmedTOTPPayload(
	ctx context.Context,
	storage operationFetcher,
	errorWrapper errors.Wrapper,
	userID uuid.UUID,
	operationToken string,
) (dto.ChangeTOTPOperation, error) {
	if userID == uuid.Nil {
		return dto.ChangeTOTPOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return dto.ChangeTOTPOperation{}, secureoperation.ErrOperationInvalid
	}

	op, err := storage.FetchOne(ctx, operationToken)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return dto.ChangeTOTPOperation{}, secureoperation.ErrOperationInvalid
		}

		return dto.ChangeTOTPOperation{}, errorWrapper.Wrap(err)
	}

	if userID != op.UserID {
		return dto.ChangeTOTPOperation{}, errors.ErrAccessForbidden
	}

	// TODO: проверить, что пользователь не заблокирован

	if op.Name != unit.NameConfirmChangeTOTP {
		return dto.ChangeTOTPOperation{}, errors.ErrAccessForbidden
	}

	if !op.Is(operationstatus.Confirmed) {
		return dto.ChangeTOTPOperation{}, secureoperation.ErrOperationIsNotConfirmed
	}

	return unit.ParseChangeTOTPPayload(op.Payload)
}
