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
		Payload           []byte    // произвольные данные операции (зависят от её типа)
		Status            operationstatus.Enum
		ExpiresAt         time.Time
	}

	// DTO - публичные данные операции, безопасные для отдачи клиенту.
	DTO struct {
		Token             string
		ConfirmMethod     confirmmethod.Enum
		RemainingAttempts int16
		RemainingResends  int16
		ResendsAt         time.Time
		ExpiresAt         time.Time
	}

	// UserDTO - данные подтверждённой операции для дальнейшей обработки прикладной логикой.
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

// WakeUp - восстанавливает операцию, загруженную из хранилища: проставляет её
// экшены и проверяет инварианты и срок действия.
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

// PublicInfo - возвращает публичные данные операции (метод подтверждения, счётчики, сроки).
// Для подтверждённой операции (без действий) метод подтверждения остаётся нулевым.
func (o *SecureOperation) PublicInfo() DTO {
	var method confirmmethod.Enum
	if len(o.actions) > 0 {
		method = o.actions[0].Method
	}

	return DTO{
		Token:             o.Token,
		ConfirmMethod:     method,
		RemainingAttempts: o.RemainingAttempts,
		RemainingResends:  o.RemainingResends,
		ResendsAt:         o.ResendsAt,
		ExpiresAt:         o.ExpiresAt,
	}
}

// UserInfo - возвращает данные операции для прикладной логики; для неподтверждённой
// операции возвращает пустой UserDTO.
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

// Is - сообщает, находится ли операция в указанном статусе.
func (o *SecureOperation) Is(status operationstatus.Enum) bool {
	return o.Status == status
}

// InitSendableAction - для текущего sendable-действия генерирует и устанавливает код
// подтверждения; для не-sendable действий (TOTP/password) не делает ничего.
func (o *SecureOperation) InitSendableAction(generateCodeFunc func() (code string, err error)) error {
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

// Notify - отправляет код подтверждения текущего sendable-действия через sendCodeFunc;
// для не-sendable действий или при отсутствии callback не делает ничего.
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

// NotifyByEmail - отправляет код подтверждения текущего sendable-действия через sendFunc,
// требуя, чтобы методом подтверждения был Email; для прочих методов возвращает ошибку.
func (o *SecureOperation) NotifyByEmail(sendFunc func(address, confirmCode string) error) error {
	return o.Notify(
		func(method confirmmethod.Enum, address, confirmCode string) error {
			if method != confirmmethod.Email {
				return errors.NewInternalError("ConfirmMethod is not yet supported", "method", method)
			}

			return sendFunc(address, confirmCode)
		},
	)
}

// FirstAction - возвращает текущее (первое неподтверждённое) действие операции.
func (o *SecureOperation) FirstAction() (first ConfirmAction, ok bool) {
	if len(o.actions) == 0 {
		return ConfirmAction{}, false
	}

	return o.actions[0], true
}

// Actions - возвращает оставшиеся неподтверждённые действия операции.
func (o *SecureOperation) Actions() []ConfirmAction {
	return o.actions
}
