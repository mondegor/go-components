package session

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// OpenSession - открытие новой сессии после подтверждённой операции авторизации.
	OpenSession struct {
		txManager           mrstorage.DBTxManager
		sessionIssuer       sessionIssuer
		storageUserActivity userActivityStatCreator
		openSessionCounter  openSessionCounter
		excessQueue         excessQueueProducer
		handlerAuthFlow     authFlowHandler
		tokenCreator        tokenCreator
		storageOperation    operationConsumer
		realmRegistry       mrauth.RealmRegistry
		logOperation        operationLogger
		logger              mrlog.Logger
		limiter             *sessionLimiter
		errorWrapper        errors.Wrapper
	}

	sessionIssuer interface {
		Issue(ctx context.Context, session entity.Session) (sessionID uint32, err error)
	}

	// openSessionCounter - считает открытые сессии пользователя в realm для контроля лимита на входе.
	openSessionCounter interface {
		FetchOpenSessionCount(ctx context.Context, userID uuid.UUID, realmID uint16) (count int, err error)
	}

	userActivityStatCreator interface {
		InsertOrUpdate(ctx context.Context, row entity.UserActivityStat) error
	}

	// excessQueueProducer - ставит пользователя в очередь на фоновую чистку лишних сессий его realm.
	excessQueueProducer interface {
		Enqueue(ctx context.Context, userID uuid.UUID, realmID uint16, sessionMax int) error
	}

	authFlowHandler interface {
		Execute(ctx context.Context, op secureoperation.SecureOperation) (userScopes dto.UserScopes, notifyAuthSuccess func(context.Context), err error)
	}

	tokenCreator interface {
		Create(ctx context.Context, userScopes dto.UserScopes) (token dto.AuthTokenPair, err error)
	}

	// operationConsumer - потребляет (удаляет) подтверждённую операцию в транзакции открытия
	// сессии, делая её одноразовым «билетом». Delete в транзакции сериализует конкурентные
	// удаления одной операции: успешно удаляет только один из конкурентов, остальные получают
	// ErrEventStorageNoRecordFound.
	operationConsumer interface {
		Delete(ctx context.Context, token string) error
	}

	// operationLogger - best-effort продюсер записей журнала защищённых операций.
	operationLogger interface {
		Log(ctx context.Context, entry entity.SecureOperationLog)
	}
)

// NewOpenSession - создаёт объект OpenSession.
func NewOpenSession(
	txManager mrstorage.DBTxManager,
	sessionIssuer sessionIssuer,
	storageUserActivity userActivityStatCreator,
	openSessionCounter openSessionCounter,
	excessQueue excessQueueProducer,
	handlerAuthFlow authFlowHandler,
	tokenCreator tokenCreator,
	storageOperation operationConsumer,
	realmRegistry mrauth.RealmRegistry,
	logOperation operationLogger,
	logger mrlog.Logger,
	allowedRealms []LimitRealm,
	softThreshold, hardThreshold int,
) *OpenSession {
	return &OpenSession{
		txManager:           txManager,
		sessionIssuer:       sessionIssuer,
		storageUserActivity: storageUserActivity,
		openSessionCounter:  openSessionCounter,
		excessQueue:         excessQueue,
		handlerAuthFlow:     handlerAuthFlow,
		tokenCreator:        tokenCreator,
		storageOperation:    storageOperation,
		realmRegistry:       realmRegistry,
		logOperation:        logOperation,
		logger:              logger,
		limiter:             newSessionLimiter(allowedRealms, softThreshold, hardThreshold),
		errorWrapper:        errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - открывает новую сессию: сохраняет сессию (с генерацией её идентификатора),
// выпускает пару токенов и фиксирует активность пользователя.
func (uc *OpenSession) Execute(ctx context.Context, meta dto.SessionMeta, op secureoperation.SecureOperation) (authToken dto.AuthTokenPair, err error) {
	if op.Type != operationtype.CreateUser && op.Type != operationtype.AuthorizeUser {
		return dto.AuthTokenPair{}, errors.ErrAccessForbidden
	}

	if !op.Is(operationstatus.Confirmed) {
		return dto.AuthTokenPair{}, mrauth.ErrOperationIsNotConfirmed
	}

	userScopes, notifyAuthSuccess, err := uc.handlerAuthFlow.Execute(ctx, op)
	if err != nil {
		return dto.AuthTokenPair{}, uc.errorWrapper.Wrap(err)
	}

	realmID, ok := uc.realmRegistry.IDByName(userScopes.Realm)
	if !ok {
		return dto.AuthTokenPair{}, errors.ErrInternalIncorrectInputData.WithDetails("realm is unknown", "realm", userScopes.Realm)
	}

	// проверка лимита сессий: при достижении soft пользователь ставится в очередь
	// на фоновую чистку, при достижении hard вход временно отклоняется
	if err = uc.applySessionLimit(ctx, userScopes.UserID, realmID, userScopes.Kind); err != nil {
		if errors.Is(err, mrauth.ErrSessionLimitExceededTryLater) {
			// жёсткий лимит сессий: фиксируем блокировку входа в журнале
			uc.logOperation.Log(
				ctx,
				entity.NewSecureOperationLog(
					userScopes.UserID,
					meta.ClientIP,
					op.Type.String(),
					op.FirstActionMethod(),
					logstatus.Blocked,
					logreason.SessionLimit,
				),
			)
		}

		return dto.AuthTokenPair{}, uc.errorWrapper.Wrap(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		// сессия выпускается ПЕРВОЙ, до токена: токен несёт как sid идентификатор сессии,
		// который возвращает Issue
		userScopes.SessionID, err = uc.sessionIssuer.Issue(
			ctx,
			entity.Session{
				UserID:    userScopes.UserID,
				UserAgent: meta.UserAgent,
				// инвариант: real IP всегда задан (источник RemoteAddr), поэтому LastIP заполнен всегда
				LastIP: meta.ClientIP.Real,
			},
		)
		if err != nil {
			return err
		}

		// выпуск токена может быть дорогим (например, подпись), а здесь он выполняется ВНУТРИ
		// транзакции, что удлиняет её и снижает пропускную способность под нагрузкой. Вынести
		// выпуск из транзакции мешает порядок: sid берётся из идентификатора сессии, который
		// возвращает Issue выше внутри этой же транзакции. Осознанный trade-off: удержание
		// транзакции на время выпуска принято ради согласованности идентификатора сессии и токена.
		// TODO: под нагрузкой вынести выпуск токена из транзакции - получать идентификатор сессии
		// заранее (вне tx), выпустить токен, затем короткая транзакция сохранения сессии и токена.
		authToken, err = uc.tokenCreator.Create(ctx, userScopes)
		if err != nil {
			return err
		}

		// операция потребляется (удаляется) последней в той же транзакции: одноразовый билет,
		// окно реплея закрыто. Delete возвращает ErrEventStorageNoRecordFound, если операции уже
		// нет (её потребил конкурентный запрос) - транзакция откатывается; конкурентные удаления
		// Delete сериализует сам (см. operationConsumer), поэтому отдельная блокирующая
		// выборка не нужна. При любом сбое выше всё откатывается, операция остаётся
		// Confirmed и вход можно безопасно повторить тем же токеном (ConfirmOperation идемпотентен)
		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			// операция потреблена конкурентным запросом: снаружи это тот же
			// «токен больше не действует», а не нарушение инварианта
			if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
				return mrauth.ErrOperationInvalid
			}

			return err
		}

		return nil
	})
	if err != nil {
		return dto.AuthTokenPair{}, uc.errorWrapper.Wrap(err)
	}

	// сессия открыта: фиксируем успешный вход в журнале (запись вне транзакции).
	// Подтверждение самой операции уже зафиксировано как CONFIRMED в ConfirmOperation,
	// поэтому здесь используется отдельный статус - иначе события неразличимы
	uc.logOperation.Log(
		ctx,
		entity.NewSecureOperationLog(
			userScopes.UserID,
			meta.ClientIP,
			op.Type.String(),
			op.FirstActionMethod(),
			logstatus.SessionOpened,
			logreason.Unspecified,
		),
	)

	// Активность пользователя пишется ВНЕ транзакции: это некритичная статистика "последнего входа".
	// Внутри транзакции её сбой откатывал бы уже выпущенные сессию и токены, а сама запись удлиняла
	// бы удержание транзакции. К этому моменту commit уже прошёл - сессия открыта
	// и токены выданы, поэтому потерю одного обновления телеметрии при транзиентном сбое хранилища
	// сознательно игнорируется, чтобы не проваливать успешный логин.
	now := time.Now().UTC()
	userActivity := entity.UserActivityStat{
		UserID:        userScopes.UserID,
		RealmID:       realmID,
		LastLoginIP:   meta.ClientIP.Real,
		LastLoggedAt:  now,
		LastVisitedAt: now,
	}

	if err = uc.storageUserActivity.InsertOrUpdate(ctx, userActivity); err != nil {
		uc.logger.Error(ctx, "failed to insert user activity stat", "user_id", userScopes.UserID, "error", err)
	}

	if notifyAuthSuccess != nil {
		notifyAuthSuccess(ctx)
	}

	return authToken, nil
}

// applySessionLimit - сигналит на фоновую чистку лишних сессий и применяет hard-лимит на вход.
// Лишние сессии не закрываются синхронно: при достижении soft пользователь ставится в очередь
// (фоновый SessionExcessTrimmer ужмёт его сессии до лимита), при достижении hard вход временно
// отклоняется, пока чистка не освободит место от лишних сессий.
//
// Контроль лимита eventual-consistent, а НЕ строгий: openCount читается вне транзакции и без
// блокировки (прежний синхронный mrlock-кулдаун намеренно убран ради дешёвого горячего пути входа).
// Поэтому при burst'е конкурентных входов одного пользователя все запросы видят одинаковый openCount
// и могут одновременно проскочить hard-гейт - число открытых сессий способно временно превысить hard.
// фоновый SessionExcessTrimmer сведёт счёт обратно к лимиту. Если потребуется строгий потолок -
// входной путь надо сериализовать (advisory-lock / счётчик).
//
// Сигнал на фоновую чистку ставится ДО hard-гейта, поэтому отказ во входе (N >= hard) тоже
// планирует чистку: иначе пользователь, перепрыгнувший hard, отклонялся бы на каждом входе без
// шанса на авто-разблокировку.
//
// Сигнал может быть ПРОПУЩЕН (никто не поставит в очередь) только в одном случае: пользователь
// стартует с M <= soft-2 живых сессий и burst'ом конкурентных входов перепрыгивает soft (а то и
// hard) за один заход - все запросы читают одинаковый N = M, для каждого N+1 = M+1 < soft, и ни
// один не ставит сигнал. Это самоизлечивается: следующий же (последовательный) вход видит
// N = M+K >= soft и ставит сигнал - даже если этот вход отклоняется по hard.
func (uc *OpenSession) applySessionLimit(ctx context.Context, userID uuid.UUID, realmID uint16, kind string) error {
	openCount, err := uc.openSessionCounter.FetchOpenSessionCount(ctx, userID, realmID)
	if err != nil {
		return err
	}

	limit, soft, hard := uc.limiter.thresholds(realmID, kind)

	// +1 - место под открываемую сессию
	if openCount+1 >= soft {
		// постановка в очередь не должна валить логин - чистка наверстает на след. входе
		if err := uc.excessQueue.Enqueue(ctx, userID, realmID, limit); err != nil {
			uc.logger.Error(ctx, "failed to enqueue user for session excess cleanup", "user_id", userID, "error", err)
		}
	}

	if openCount >= hard {
		return mrauth.ErrSessionLimitExceededTryLater
	}

	return nil
}
