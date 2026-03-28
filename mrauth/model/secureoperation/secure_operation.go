package secureoperation

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

type (
	// SecureOperation - операция, проводимая пользователем требующая
	// от него подтверждения своей личности каким-либо способом.
	SecureOperation struct {
		Token             string
		Name              string
		UserID            uuid.UUID
		actions           []ConfirmAction
		RemainingAttempts int16     // кол-во оставшихся попыток подтверждения текущего экшена операции
		RemainingResends  int16     // кол-во оставшихся попыток повторной отправки кода подтверждения
		ResendsAt         time.Time // время, начиная с которого можно сделать повторную отправку кода подтверждения
		Payload           []byte    // audience, visitorId
		Status            operationstatus.Enum
		ExpiresAt         time.Time

		// sendCode func(address, confirmCode string) error
	}

	// DTO - comment struct.
	DTO struct {
		Token             string
		ConfirmMethod     confirmmethod.Enum
		RemainingAttempts int16
		RemainingResends  int16
		ResendsAt         time.Time
		ExpiresAt         time.Time
	}

	// UserDTO - comment struct.
	UserDTO struct {
		Token   string
		UserID  uuid.UUID
		Payload []byte
	}
)

// NewOperation - создаёт объект SecureOperation.
func NewOperation(
	token string,
	name string,
	userID uuid.UUID,
	actions []ConfirmAction,
	payload []byte,
) (SecureOperation, error) {
	op := SecureOperation{
		Name:    name,
		UserID:  userID,
		actions: actions,
		Payload: payload,
		Status:  operationstatus.Opened,
	}

	if err := op.checkInvariants(); err != nil {
		return SecureOperation{}, err
	}

	if err := op.ActivateConfirmation(token); err != nil {
		return SecureOperation{}, err
	}

	return op, nil
}

// WakeUp - comment method.
func WakeUp(op *SecureOperation, actions []ConfirmAction) error {
	if op == nil {
		return errors.ErrInternalNilPointer.New("op is nil")
	}

	if op.Token == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("token is empty")
	}

	op.actions = actions

	if err := op.checkInvariants(); err != nil {
		return err
	}

	if time.Now().After(op.ExpiresAt) {
		return ErrOperationAlreadyExpired
	}

	return nil
}

// инварианты:
// 1. У операции в статусе Opened должен быть хотя бы один неподтверждённый ConfirmAction (все экшены до него должны быть подтверждёнными)
// 2. У операции в статусе Confirmed не должно быть экшенов.
func (o *SecureOperation) checkInvariants() error {
	if o.Name == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("name is empty")
	}

	if o.Status == operationstatus.Confirmed {
		if len(o.actions) > 0 {
			return errors.ErrInternalIncorrectInputData.WithDetails("operation is confirmed, but len(actions) > 0")
		}

		return nil
	}

	if o.Status != operationstatus.Opened {
		return errors.ErrInternalIncorrectInputData.WithDetails("operation status is unknown")
	}

	if len(o.actions) == 0 {
		return errors.ErrInternalIncorrectInputData.WithDetails("operation is opened, but len(actions) == 0")
	}

	for i, action := range o.actions {
		if action.Method == 0 {
			return errors.ErrInternalIncorrectInputData.WithDetails("action without method", "index", i)
		}
	}

	return nil
}

// PublicInfo - comment method.
func (o *SecureOperation) PublicInfo() DTO {
	return DTO{
		Token:             o.Token,
		ConfirmMethod:     o.actions[0].Method,
		RemainingAttempts: o.RemainingAttempts,
		RemainingResends:  o.RemainingResends,
		ResendsAt:         o.ResendsAt,
		ExpiresAt:         o.ExpiresAt,
	}
}

// UserInfo - comment method.
func (o *SecureOperation) UserInfo() UserDTO {
	if o.Status == operationstatus.Confirmed {
		return UserDTO{
			Token:   o.Token,
			UserID:  o.UserID,
			Payload: o.Payload,
		}
	}

	return UserDTO{}
}

// Is - сообщает, находится ли операция в указанно статусе.
func (o *SecureOperation) Is(status operationstatus.Enum) bool {
	return o.Status == status
}

// InitConfirmCode - comment method.
func (o *SecureOperation) InitConfirmCode(generateCodeFunc func() (code string, err error)) error {
	if o.Status != operationstatus.Opened || len(o.actions) == 0 {
		return errors.New("operation is not opened")
	}

	if !o.actions[0].Sendable() {
		return nil
	}

	if generateCodeFunc == nil {
		return errors.New("generateCode is nil")
	}

	code, err := generateCodeFunc()
	if err != nil {
		return err
	}

	o.actions[0].ConfirmCode = code

	return nil
}

// Notify - comment method.
func (o *SecureOperation) Notify(
	sendCodeFunc func(method confirmmethod.Enum, address, confirmCode string) error,
) error {
	if o.Status != operationstatus.Opened || len(o.actions) == 0 {
		return errors.New("operation is not opened")
	}

	if sendCodeFunc == nil || !o.actions[0].Sendable() {
		return nil
	}

	if o.actions[0].Address == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("address is empty")
	}

	if o.actions[0].ConfirmCode == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("confirmCode is empty")
	}

	return sendCodeFunc(
		o.actions[0].Method,
		o.actions[0].Address,
		o.actions[0].ConfirmCode,
	)
}

// Actions - comment method.
func (o *SecureOperation) Actions() []ConfirmAction {
	return o.actions
}
