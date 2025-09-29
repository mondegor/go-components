package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/enum"
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
		Actions           []ConfirmAction
		RemainingAttempts uint32    // кол-во оставшихся попыток подтверждения текущего экшена операции
		RemainingResends  uint32    // кол-во оставшихся попыток повторной отправки кода подтверждения
		ResendsAt         time.Time // время, начиная с которого можно сделать повторную отправку кода подтверждения
		Payload           []byte    // audience, visitorId
		Status            enum.OperationStatus
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

	// ConfirmAction - comment struct.
	ConfirmAction struct {
		Method        enum.ConfirmMethod `json:"method"` // email (отправить событие), password, phone (отправить событие), TOTP
		MaxAttempts   uint32             `json:"maxAttempts"`
		MaxResends    uint32             `json:"maxResends,omitempty"`
		MinResendTime time.Duration      `json:"minResendTime,omitempty"`
		Expiry        time.Duration      `json:"expiry"`
		Address       string             `json:"address,omitempty"`

		// omitempty - ????, hash(пароль) брать у юзера, hash(TOTP) брать у юзера, email одноразовый код, phone одноразовый код
		Secret    string `json:"secret,omitempty"`
		Confirmed bool   `json:"confirmed"`
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
		ConfirmMethod enum.ConfirmMethod
		LogStatus     string // TODO: to status
		CreatedAt     time.Time
	}

	// co.eventEmitter.Emit(ctx, "CreateRequest", mrargs.Group{"userEmail": parsedLogin.Value, "secretCode": secretCode}).

	// global log operations: session bad.
)

// ErrOperationHasOnlyConfirmedActions - operation has only confirmed actions.
var ErrOperationHasOnlyConfirmedActions = mrerr.NewKindInternal("operation has only confirmed actions")

// NextNotConfirmedAction - comments method.
func (so *SecureOperation) NextNotConfirmedAction() (*ConfirmAction, error) {
	if so.Status != enum.OperationStatusOpened {
		return nil, mr.ErrInternal.New().WithAttr("details", "operation status must be OPENED")
	}

	if len(so.Actions) == 0 {
		return nil, mr.ErrInternal.New().WithAttr("details", "operation does not contain any actions")
	}

	for i := range so.Actions {
		if so.Actions[i].Confirmed {
			continue
		}

		if so.Actions[i].Method == 0 {
			return nil, mr.ErrInternalUnexpectedValue.New(fmt.Sprintf("so.Actions[%d].Method", i), 0)
		}

		return &so.Actions[i], nil
	}

	return nil, ErrOperationHasOnlyConfirmedActions.New()
}
