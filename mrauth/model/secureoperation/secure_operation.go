package secureoperation

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
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

	if time.Now().UTC().After(op.ExpiresAt) {
		return mrauth.ErrOperationAlreadyExpired
	}

	return nil
}

// инварианты:
// 1. У операции в статусе Opened должен быть хотя бы один неподтверждённый ConfirmAction (все экшены до него должны быть подтверждёнными)
// 2. У операции в статусе Confirmed не должно быть экшенов.
// 3. Sendable-действия идут раньше не-sendable: код подтверждения отправляется только по цепочке вперёд.
// 4. Аварийный код принимается только последним действием цепочки (см. ниже).
// 5. Признак приёма аварийного кода не совместим с sendable-действием.
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
		isLast := i == len(o.actions)-1

		if action.Method == 0 {
			return errors.ErrInternalIncorrectInputData.WithDetails("action without method", "index", i)
		}

		// цепочка идёт от sendable-действий к не-sendable: код подтверждения генерится и отправляется
		// при переходе к следующему действию, поэтому sendable после не-sendable не бывает.
		// Цепочка из двух не-sendable (2FA + аварийный код) при этом допустима
		if action.Sendable() && i > 0 && !o.actions[i-1].Sendable() {
			return errors.ErrInternalIncorrectInputData.WithDetails("sendable action must precede non-sendable", "index", i)
		}

		// аварийный код принимается только последним действием - и как замена его основного
		// доказательства (AllowRecovery), и как отдельное действие. На этом держится правило
		// "за операцию расходуется не более одного аварийного кода": предъявить его раньше
		// последнего действия нельзя, а значит нельзя и погасить код, не завершив комбинацию
		if !isLast && (action.AllowRecovery || action.Method == confirmmethod.Recovery) {
			return errors.ErrInternalIncorrectInputData.WithDetails("recovery code is accepted by the last action only", "index", i)
		}

		// аварийный код подменяет только доказательство второго фактора: у sendable-действия
		// код сверяет сама операция, а не верификатор, поэтому признак там ничего не значит
		// и выставленный по ошибке молча замаскировал бы неверно собранную цепочку
		if action.Sendable() && action.AllowRecovery {
			return errors.ErrInternalIncorrectInputData.WithDetails("sendable action cannot allow recovery", "index", i)
		}
	}

	return nil
}

// Is - сообщает, находится ли операция в указанном статусе.
func (o *SecureOperation) Is(status operationstatus.Enum) bool {
	return o.Status == status
}

// InitSendableAction - для текущего sendable-действия генерирует код подтверждения:
// хеш сохраняется в ConfirmCode (попадает в хранилище), открытый код - в PlainConfirmCode
// (только для отправки). Для не-sendable действий (TOTP/password) не делает ничего.
func (o *SecureOperation) InitSendableAction(generateCodeFunc func() (code, hashedCode string, err error)) error {
	if o.Status != operationstatus.Opened || len(o.actions) == 0 {
		return errors.ErrInternalIncorrectInputData.WithDetails(
			"operation is not opened", "status", o.Status, "actions", len(o.actions),
		)
	}

	if !o.actions[0].Sendable() {
		return nil
	}

	if generateCodeFunc == nil {
		return errors.ErrInternalNilPointer.New("generateCodeFunc is nil")
	}

	code, hashedCode, err := generateCodeFunc()
	if err != nil {
		return err
	}

	o.actions[0].ConfirmCode = hashedCode
	o.actions[0].PlainConfirmCode = code

	return nil
}

// Notify - отправляет код подтверждения текущего sendable-действия через sendCodeFunc;
// для не-sendable действий или при отсутствии callback не делает ничего.
func (o *SecureOperation) Notify(
	sendCodeFunc func(method confirmmethod.Enum, address, confirmCode string) error,
) error {
	if o.Status != operationstatus.Opened || len(o.actions) == 0 {
		return errors.ErrInternalIncorrectInputData.WithDetails(
			"operation is not opened", "status", o.Status, "actions", len(o.actions),
		)
	}

	if sendCodeFunc == nil || !o.actions[0].Sendable() {
		return nil
	}

	if o.actions[0].Address == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("address is empty")
	}

	if o.actions[0].PlainConfirmCode == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("confirmCode is empty")
	}

	return sendCodeFunc(
		o.actions[0].Method,
		o.actions[0].Address,
		o.actions[0].PlainConfirmCode,
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

// Actions - возвращает оставшиеся неподтверждённые действия операции.
func (o *SecureOperation) Actions() []ConfirmAction {
	return o.actions
}

// FirstAction - возвращает текущее (первое неподтверждённое) действие операции.
func (o *SecureOperation) FirstAction() (first ConfirmAction, ok bool) {
	if len(o.actions) == 0 {
		return ConfirmAction{}, false
	}

	return o.actions[0], true
}

// FirstActionMethod - возвращает метод текущего (первого неподтверждённого) действие операции.
// Если действий нет, то возвращается confirmmethod.Unspecified.
func (o *SecureOperation) FirstActionMethod() confirmmethod.Enum {
	if len(o.actions) == 0 {
		return confirmmethod.Unspecified
	}

	return o.actions[0].Method
}
