package secureoperation

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

type (
	// SecureOperation - операция, проводимая пользователем требующая
	// от него подтверждения своей личности каким-либо способом.
	SecureOperation struct {
		Token             string
		Name              string
		UserID            uuid.UUID
		actionIndex       int
		Actions           []ConfirmAction
		RemainingAttempts uint32    // кол-во оставшихся попыток подтверждения текущего экшена операции
		RemainingResends  uint32    // кол-во оставшихся попыток повторной отправки кода подтверждения
		ResendsAt         time.Time // время, начиная с которого можно сделать повторную отправку кода подтверждения
		Payload           []byte    // audience, visitorId
		Status            operationstatus.Enum
		ExpiresAt         time.Time
	}

	// // CreateOperation - сообщение для получателя.
	// CreateOperation struct {
	// 	Name       string
	// 	UserID     uuid.UUID
	// 	Address    contactaddress.ContactAddress
	// 	UseAuth2FA bool
	// 	Payload    map[string]string
	// }.

	// // CreateRequestResult - comment struct.
	// CreateRequestResult struct {
	// 	Token      string
	// 	UserEmail  string
	// 	SecretCode string
	// }.

	// co.eventEmitter.Emit(ctx, "CreateRequest", conv.Group{"userEmail": parsedLogin.Value, "secretCode": secretCode}).

	// global log operations: session bad.
)

// NewAnonymus - comment func.
func NewAnonymus(token, name string, actions []ConfirmAction, payload []byte) (SecureOperation, error) {
	return New(token, name, uuid.Nil, actions, payload)
}

// New - comment func.
func New(token, name string, userID uuid.UUID, actions []ConfirmAction, payload []byte) (SecureOperation, error) {
	index, err := nextConfirmingAction(actions)
	if err != nil {
		return SecureOperation{}, errors.New("actions is empty")
	}

	return SecureOperation{
		Token:             token,
		Name:              name,
		UserID:            userID,
		actionIndex:       index,
		Actions:           actions,
		RemainingAttempts: actions[index].MaxAttempts,
		RemainingResends:  actions[index].MaxResends,
		ResendsAt:         time.Now().Add(actions[index].MinResendTime).Round(1 * time.Second),
		Payload:           payload,
		Status:            operationstatus.Opened,
		ExpiresAt:         time.Now().Add(actions[index].Expiry).Round(1 * time.Second),
	}, nil
}

// nextConfirmingAction - comments func.
func nextConfirmingAction(actions []ConfirmAction) (index int, err error) {
	if len(actions) == 0 {
		return -1, errors.NewInternalError("operation does not contain any actions")
	}

	for i := range actions {
		if actions[i].Confirmed {
			continue
		}

		if actions[i].Method == 0 {
			return -1, errors.NewInternalError(
				"operation contains action without method",
				"index", i,
			)
		}

		return i, nil
	}

	return -1, errors.NewInternalError("operation has only confirmed actions")
}
