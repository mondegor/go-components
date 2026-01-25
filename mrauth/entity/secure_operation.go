package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

const (
	// ModelNameSecureOperation - название сущности.
	ModelNameSecureOperation = "mrauth.SecureOperation"
)

type (
	// SecureOperation - сообщение для получателя.
	SecureOperation struct {
		Token             string
		Name              string
		UserID            uuid.UUID
		Actions           []dto.ConfirmAction
		RemainingAttempts uint32    // кол-во оставшихся попыток подтверждения текущего экшена операции
		RemainingResends  uint32    // кол-во оставшихся попыток повторной отправки кода подтверждения
		ResendsAt         time.Time // время, начиная с которого можно сделать повторную отправку кода подтверждения
		Payload           []byte    // audience, visitorId
		Status            operationstatus.Enum
		ExpiresAt         time.Time
	}

	// CreateOperation - сообщение для получателя.
	CreateOperation struct {
		Name       string
		UserID     uuid.UUID
		Address    contactaddress.ContactAddress
		UseAuth2FA bool
		Payload    map[string]string
	}

	// CreateRequestResult - comment struct.
	CreateRequestResult struct {
		Token      string
		UserEmail  string
		SecretCode string
	}

	// SecureOperationLog - сообщение для получателя.
	SecureOperationLog struct {
		RecordID      uint64
		VisitorID     uuid.UUID
		OperationName string
		ConfirmMethod confirmmethod.Enum
		LogStatus     string // TODO: to status
		CreatedAt     time.Time
	}

	// co.eventEmitter.Emit(ctx, "CreateRequest", conv.Group{"userEmail": parsedLogin.Value, "secretCode": secretCode}).

	// global log operations: session bad.
)

// ErrOperationHasOnlyConfirmedActions - operation has only confirmed actions.
var (
	ErrOperationHasOnlyConfirmedActions = errors.NewInternalProto("operation has only confirmed actions")
)

// NextNotConfirmedAction - comments method.
func (so *SecureOperation) NextNotConfirmedAction() (*dto.ConfirmAction, error) {
	if so.Status != operationstatus.Opened {
		return nil, errors.NewInternalError("operation status must be OPENED")
	}

	if len(so.Actions) == 0 {
		return nil, errors.NewInternalError("operation does not contain any actions")
	}

	for i := range so.Actions {
		if so.Actions[i].Confirmed {
			continue
		}

		if so.Actions[i].Method == 0 {
			return nil, errors.NewInternalError(
				"operation contains action without method",
				"index", i,
			)
		}

		return &so.Actions[i], nil
	}

	return nil, ErrOperationHasOnlyConfirmedActions.New()
}
