package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlock"
	"github.com/mondegor/go-core/mrtype"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

const (
	createUserLockKeyPrefix = "auth.create-user:"
)

type (
	// CreateUser - usecase создания пользователя с подтверждением через защищённую операцию.
	CreateUser struct {
		opener          operationOpener
		userChecker     userLoginChecker
		factory2FA      user2faActionCreator
		locker          mrlock.Locker
		logOperation    operationLogger
		errorWrapper    errors.Wrapper
		realm2operation map[string]createUserOperation
	}

	// CreateRealmUser - сопоставление realm с операцией создания пользователя для него.
	CreateRealmUser struct {
		Name      string
		Operation createUserOperation
	}

	createUserOperation interface {
		// Name - имя создаваемой операции; используется для событий журнала, возникающих
		// до её создания (pre-op), чтобы они не разъезжались с именем самой операции.
		Name() string

		// Expiry - срок действия отправляемого кода подтверждения; по нему выставляется окно
		// троттла повторной регистрации, чтобы оно не разъезжалось с настройкой самой операции.
		Expiry() time.Duration

		Create(
			user2FA dto.User2FA,
			langCode string,
			timeZone string,
			userEmail contactaddress.ContactAddress,
			registeredIP mrtype.DetailedIP,
		) (secureoperation.SecureOperation, error)
	}

	user2faActionCreator interface {
		CreateByUserLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (dto.User2FA, error)
	}

	// operationLogger - best-effort продюсер записей журнала защищённых операций.
	operationLogger interface {
		Log(ctx context.Context, entry entity.SecureOperationLog)
	}
)

// NewCreateUser - создаёт объект CreateUser.
func NewCreateUser(
	opener operationOpener,
	userChecker userLoginChecker,
	factory2FA user2faActionCreator,
	locker mrlock.Locker,
	logOperation operationLogger,
	allowedRealms []CreateRealmUser,
) *CreateUser {
	realm2operation := make(map[string]createUserOperation, len(allowedRealms))
	for _, realm := range allowedRealms {
		realm2operation[realm.Name] = realm.Operation
	}

	return &CreateUser{
		opener:          opener,
		userChecker:     userChecker,
		factory2FA:      factory2FA,
		locker:          locker,
		logOperation:    logOperation,
		errorWrapper:    errors.NewServiceOperationFailedWrapper(),
		realm2operation: realm2operation,
	}
}

// Execute - инициирует создание пользователя: открывает защищённую операцию подтверждения по коду
// и отправляет код на email. registeredIP фиксируется в payload операции как IP регистрации.
//
// Язык и часовой пояс приходят уже определёнными по запросу и кладутся
// в payload операции как есть: это значения из списков, зарегистрированных приложением, поэтому
// подбирать здесь нечего. В payload они фиксируются на момент заявки, чтобы к моменту
// подтверждения email профиль создавался с теми же настройками, с какими шла регистрация.
func (co *CreateUser) Execute(
	ctx context.Context,
	realm, langCode, timeZone string,
	userEmail contactaddress.ContactAddress,
	registeredIP mrtype.DetailedIP,
) (op secureoperation.SecureOperation, err error) {
	if langCode == "" {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("langCode is empty")
	}

	if timeZone == "" {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("timeZone is empty")
	}

	if userEmail.Value() == "" {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userEmail is empty")
	}

	opCreator, ok := co.realm2operation[realm]
	if !ok {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("realm is unknown", "realm", realm)
	}

	// лок держится столько же, сколько действителен отправленный код, и НЕ освобождается при
	// успехе - это намеренный анти-спам троттл повторной отправки кода подтверждения на тот же
	// email. Сама операция может пережить лок (повторная отправка кода и 2FA-шаг продлевают её
	// ExpiresAt), поэтому троттл ограничивает частоту отправок, а не время жизни операции
	unlockEmail, err := co.locker.LockWithExpiry(ctx, createUserLockKeyPrefix+realm+":"+userEmail.Value(), opCreator.Expiry())
	if err != nil {
		if errors.Is(err, mrlock.ErrLockKeyNotObtained) {
			// анти-спам троттл повторной регистрации: фиксируем в журнале заблокированную попытку
			// (операция не создана, поэтому её имя берётся у фабрики, а метод подтверждения неизвестен)
			co.logOperation.Log(
				ctx,
				entity.NewSecureOperationLog(
					uuid.Nil,
					registeredIP,
					opCreator.Name(),
					confirmmethod.Unspecified,
					logstatus.Blocked,
					logreason.Throttled,
				),
			)

			// создаётся ошибка со сроком повторной попытки, равным верхней границе окна троттла:
			// сколько его осталось на самом деле, mrlock не сообщает
			return secureoperation.SecureOperation{},
				mrauth.ErrSignupAlreadyInProgressTryLater.Wrap(
					mrauth.NewRetryAfterError(opCreator.Expiry()),
				)
		}

		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	defer func() {
		// если в процессе выполнения метода возникла ошибка,
		// то емаил освобождается
		if err != nil {
			unlockEmail()
		}
	}()

	if err = co.userChecker.CheckAvailabilityRealm(ctx, realm, userEmail); err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	// если email уже принадлежит существующему пользователю, его 2FA будет добавлен вторым шагом
	// операции (для нового email пользователь не найден и используется пустой User2FA)
	user2FA, err := co.factory2FA.CreateByUserLogin(ctx, userEmail)
	if err != nil {
		if !errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
		}
	}

	op, err = opCreator.Create(user2FA, langCode, timeZone, userEmail, registeredIP)
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	// поток регистрации анонимный, форензику несёт IP; если же email принадлежит
	// существующему пользователю, владелец операции известен и Open зафиксирует в журнале его
	err = co.opener.Open(
		ctx,
		dto.ActorMeta{ClientIP: registeredIP},
		op,
		"confirm.user.activation",
		conv.Group{"lang": langCode},
	)
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	return op, nil
}
