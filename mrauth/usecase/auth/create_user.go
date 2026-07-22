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
	createUserLockTimeout   = 10 * time.Minute
)

type (
	// CreateUser - usecase создания пользователя с подтверждением через защищённую операцию.
	CreateUser struct {
		opener           operationOpener
		userChecker      userLoginChecker
		factory2FA       user2faActionCreator
		locker           mrlock.Locker
		logOperation     operationLogger
		timeZoneResolver timeZoneResolver
		errorWrapper     errors.Wrapper
		realm2operation  map[string]createUserOperation
	}

	// CreateUserRealm - сопоставление realm с операцией создания пользователя для него.
	CreateUserRealm struct {
		Name      string
		Operation createUserOperation
	}

	createUserOperation interface {
		// Name - имя создаваемой операции; используется для событий журнала, возникающих
		// до её создания (pre-op), чтобы они не разъезжались с именем самой операции.
		Name() string
		Create(
			user2FA dto.User2FA,
			langCode string,
			timeZone string,
			address contactaddress.ContactAddress,
			registeredIP mrtype.DetailedIP,
		) (secureoperation.SecureOperation, error)
	}

	user2faActionCreator interface {
		CreateByUserLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (dto.User2FA, error)
	}

	// timeZoneResolver - подбирает пояс, зарегистрированный в приложении.
	timeZoneResolver interface {
		Resolve(in dto.TimeZoneInfo) (name string)
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
	timeZoneResolver timeZoneResolver,
	allowedRealms []CreateUserRealm,
) *CreateUser {
	realm2operation := make(map[string]createUserOperation, len(allowedRealms))
	for _, realm := range allowedRealms {
		realm2operation[realm.Name] = realm.Operation
	}

	return &CreateUser{
		opener:           opener,
		userChecker:      userChecker,
		factory2FA:       factory2FA,
		locker:           locker,
		logOperation:     logOperation,
		timeZoneResolver: timeZoneResolver,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
		realm2operation:  realm2operation,
	}
}

// Execute - инициирует создание пользователя: открывает защищённую операцию подтверждения по коду
// и отправляет код на email. registeredIP фиксируется в payload операции как IP регистрации.
//
// Часовой пояс подбирается сразу и попадает в payload операции уже разрешённым IANA-именем:
// присланная клиентом пара (смещение, признак летнего времени) описывает его состояние
// на момент заявки, поэтому в payload кладётся результат подбора, а не сама пара -
// иначе к моменту подтверждения email она могла бы описывать уже другое состояние.
func (co *CreateUser) Execute(
	ctx context.Context,
	realm, langCode string,
	timeZone dto.TimeZoneInfo,
	userEmail string,
	registeredIP mrtype.DetailedIP,
) (op secureoperation.SecureOperation, err error) {
	opCreator, ok := co.realm2operation[realm]
	if !ok {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("realm is unknown")
	}

	resolvedTimeZone := co.timeZoneResolver.Resolve(timeZone)

	parsedLogin, err := contactaddress.ParseEmail(userEmail)
	if err != nil {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New(err)
	}

	// лок держится до createUserLockTimeout и НЕ освобождается при успехе - это намеренный
	// анти-спам троттл повторной отправки кода подтверждения на тот же email
	unlockEmail, err := co.locker.LockWithExpiry(ctx, createUserLockKeyPrefix+realm+":"+parsedLogin.Value(), createUserLockTimeout)
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

			return secureoperation.SecureOperation{}, mrauth.ErrSignupAlreadyInProgressTryLater
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

	if err = co.userChecker.CheckAvailabilityRealm(ctx, realm, parsedLogin); err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	// если email уже принадлежит существующему пользователю, его 2FA будет добавлен вторым шагом
	// операции (для нового email пользователь не найден и используется пустой User2FA)
	user2FA, err := co.factory2FA.CreateByUserLogin(ctx, parsedLogin)
	if err != nil {
		if !errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
		}
	}

	op, err = opCreator.Create(user2FA, langCode, resolvedTimeZone, parsedLogin, registeredIP)
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
