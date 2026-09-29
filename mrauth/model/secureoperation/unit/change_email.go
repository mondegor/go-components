package unit

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

const (
	// defaultChangeEmailExpiry - срок жизни операции подтверждения нового емаила по умолчанию.
	defaultChangeEmailExpiry = 72 * time.Hour
)

type (
	// ChangeEmail - фабрика операции второго шага смены емаила: подтверждения
	// владения новым адресом.
	ChangeEmail struct {
		actionCreator  confirmByAddressCreator
		tokenGenerator mrauth.TokenGenerator
		codeGenerator  mrauth.CodeGenerator
	}
)

// NewChangeEmail - создаёт объект ChangeEmail. Срок жизни операции expiry больше порога
// продления модели отсчитывается от её создания и не продлевается ни повторной отправкой кода,
// ни подтверждением, более короткий продлевается, как у остальных операций; 0 означает срок
// по умолчанию. Опции confirmByEmailOpts настраивают попытки и повторные отправки кода, срок
// ими не переопределяется.
func NewChangeEmail(
	tokenGenerator mrauth.TokenGenerator,
	codeGenerator mrauth.CodeGenerator,
	expiry time.Duration,
	confirmByEmailOpts ...action.Option,
) *ChangeEmail {
	if expiry < 1 {
		expiry = defaultChangeEmailExpiry
	}

	// срок идёт последним, чтобы переданные опции его не перекрыли
	opts := make([]action.Option, 0, len(confirmByEmailOpts)+1)
	opts = append(opts, confirmByEmailOpts...)
	opts = append(opts, action.WithExpiry(expiry))

	return &ChangeEmail{
		actionCreator:  action.NewConfirmByEmail(opts...),
		tokenGenerator: tokenGenerator,
		codeGenerator:  codeGenerator,
	}
}

// Create - создаёт операцию подтверждения владения новым емаилом: единственное звено -
// код на новый адрес. Второй фактор повторно не запрашивается: он предъявлен на первом
// шаге (операция operationtype.ChangeEmail), из payload которого и берутся оба адреса.
func (o *ChangeEmail) Create(userID uuid.UUID, in dto.ChangeEmailOperation) (secureoperation.SecureOperation, error) {
	if userID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userID is empty")
	}

	// адрес проверен на границе ввода первого шага, поэтому ошибка разбора - нарушение инварианта
	newEmail, err := contactaddress.ParseEmail(in.NewEmail)
	if err != nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithError(err, "payload: newEmail is not an email address")
	}

	payload, err := BuildChangeEmailPayload(in)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	operationToken, err := o.tokenGenerator.GenToken()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmCode, hashedCode, err := o.codeGenerator.GenCodeWithHash()
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	confirmAction, err := o.actionCreator.Create(newEmail, confirmCode, hashedCode)
	if err != nil {
		return secureoperation.SecureOperation{}, err
	}

	return secureoperation.NewOperation(
		operationToken,
		operationtype.ChangeEmailConfirm,
		userID,
		[]secureoperation.ConfirmAction{confirmAction},
		payload,
	)
}
