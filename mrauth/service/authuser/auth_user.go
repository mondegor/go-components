package authuser

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/notice"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrnotifier"
)

// errUserAlreadyInRealm - внутренний маркер повторной привязки пользователя к realm
// (идемпотентность повторного подтверждения); за пределы пакета не выходит.
var errUserAlreadyInRealm = errors.New("user already registered in realm")

type (
	// Service - сервис пользователя в потоке аутентификации:
	// создание пользователя/realm и подготовка его scopes к авторизации.
	//
	// События уведомлений:
	//
	//   Триггер                                  Событие                             Когда
	//   ---------------------------------------- ----------------------------------- ---------------------
	//   новый аккаунт (registerNewUser)          user.registration.success.<realm>   commit создания юзера
	//                                            user.was.registered (админ)         commit создания юзера
	//   сущ. юзер, новый realm (bindUserToRealm) user.registration.success.<realm>   commit привязки
	//   успешный вход (PrepareAuthorization)     user.authorization.success.<realm>  ПОСЛЕ commit сессии*
	//
	// * authorization.success откладывается: PrepareAuthorization возвращает callback, который
	//   OpenSession вызывает после commit'а транзакции сессии (после hard-гейта). registration/
	//   was.registered шлются сразу на своём commit'е (идемпотентны - ровно один раз на realm).
	//
	// Суффикс <realm> в ключах событий формируется через notice.KeyByEventAndRealm.
	Service struct {
		txManager        mrstorage.DBTxManager
		storageUser      userStorage
		storageUserRealm userRealmStorage
		realmRegistry    mrauth.RealmRegistry
		notifierAPI      mrnotifier.NoteProducer
		errorWrapper     errors.Wrapper
		logger           mrlog.Logger
	}

	userStorage interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (entity.User, error)
		FetchOneByLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (entity.User, error)
		Insert(ctx context.Context, row entity.ExtendedUser) error
	}

	userRealmStorage interface {
		FetchOne(ctx context.Context, userID uuid.UUID, realmID uint16) (row entity.UserRealm, err error)
		Insert(ctx context.Context, row entity.UserRealm) error
	}
)

// New - создаёт объект Service.
func New(
	txManager mrstorage.DBTxManager,
	storageUser userStorage,
	storageUserRealm userRealmStorage,
	realmRegistry mrauth.RealmRegistry,
	notifierAPI mrnotifier.NoteProducer,
	logger mrlog.Logger,
) *Service {
	return &Service{
		txManager:        txManager,
		storageUser:      storageUser,
		storageUserRealm: storageUserRealm,
		realmRegistry:    realmRegistry,
		notifierAPI:      notifierAPI,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
		logger:           logger,
	}
}

// ResolveUser - разрешает пользователя подтверждённой операции создания и возвращает его идентификатор:
// создаёт нового пользователя (с привязкой к realm) либо привязывает уже существующего к новому realm.
// Повторное подтверждение (пользователь уже привязан к этому realm) трактуется как успех - метод
// идемпотентен, поэтому вызывающему не нужно знать про внутренний маркер already-registered.
//
// userID берётся из подтверждённой операции: Nil означает, что на момент её создания email не
// принадлежал никому. Существование пользователя с этим email на момент подтверждения означает
// ретрай тем же токеном ЛИБО параллельный кросс-realm signup (лок при создании операции - пер
// realm+email, поэтому разные realm идут независимо) - в этом случае используется существующий id.
func (s *Service) ResolveUser(ctx context.Context, userID uuid.UUID, in dto.CreateUserOperation) (uuid.UUID, error) {
	// пользователь уже известен из операции - только гарантируем привязку к нужному realm
	if userID != uuid.Nil {
		return userID, s.bindUserToRealm(ctx, userID, in)
	}

	existingUser, err := s.storageUser.FetchOneByLogin(ctx, contactaddress.NewEmail(in.Email))
	if err != nil {
		// email никому не принадлежит - создаём нового пользователя
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return s.registerNewUser(ctx, in)
		}

		return uuid.Nil, s.errorWrapper.Wrap(err)
	}

	// пользователь уже создан предыдущим частичным вызовом тем же токеном либо параллельным
	// кросс-realm signup - используем существующего и гарантируем его привязку к realm
	return existingUser.ID, s.bindUserToRealm(ctx, existingUser.ID, in)
}

// PrepareAuthorization - подготавливает пользователя к авторизации: загружает его и привязку к realm,
// возвращает scopes и отложенный callback отправки login-alert'а об успешной авторизации.
// Само уведомление здесь НЕ отправляется: оно должно уйти только после успешного открытия сессии,
// поэтому вызывающий дёргает callback после commit'а.
func (s *Service) PrepareAuthorization(ctx context.Context, userID uuid.UUID, in dto.AuthorizeUserOperation) (dto.UserScopes, func(context.Context), error) {
	if userID == uuid.Nil {
		return dto.UserScopes{}, nil, errors.ErrInternalIncorrectInputData.WithDetails("userID is zero")
	}

	realmID, ok := s.realmRegistry.IDByName(in.Realm)
	if !ok {
		return dto.UserScopes{}, nil, errors.ErrInternalIncorrectInputData.WithDetails("realm is unknown", "realm", in.Realm)
	}

	user, err := s.storageUser.FetchOne(ctx, userID)
	if err != nil {
		return dto.UserScopes{}, nil, s.errorWrapper.Wrap(err, "userId", userID)
	}

	userRealm, err := s.storageUserRealm.FetchOne(ctx, userID, realmID)
	if err != nil {
		return dto.UserScopes{}, nil, s.errorWrapper.Wrap(err, "userId", userID, "realm", in.Realm)
	}

	notifyAuthSuccess := func(ctx context.Context) {
		s.notify(ctx, notice.KeyByEventAndRealm("user.authorization.success", in.Realm), conv.Group{
			"lang": in.LangCode,
			"to":   user.Email,
		})
	}

	return dto.UserScopes{
		UserID:   user.ID,
		Realm:    in.Realm,
		Kind:     userRealm.Kind,
		LangCode: user.LangCode,
	}, notifyAuthSuccess, nil
}

// registerNewUser - создаёт нового пользователя и его привязку к realm в одной транзакции,
// возвращая сгенерированный идентификатор.
func (s *Service) registerNewUser(ctx context.Context, in dto.CreateUserOperation) (uuid.UUID, error) {
	userID := uuid.New() // TODO: реализовать через интерфейс

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		err := s.storageUser.Insert(
			ctx,
			entity.ExtendedUser{
				User: entity.User{
					ID:       userID,
					Email:    in.Email,
					LangCode: in.LangCode,
					Status:   userstatus.Enabled,
				},
				RegisteredIP: in.RegisteredIP,
			},
		)
		if err != nil {
			return s.errorWrapper.Wrap(err)
		}

		return s.insertUserRealm(ctx, userID, in)
	})
	if err != nil {
		return uuid.Nil, err
	}

	// новый аккаунт: уведомление о регистрации юзеру
	s.notify(ctx, notice.KeyByEventAndRealm("user.registration.success", in.Realm), conv.Group{
		"lang": in.LangCode,
		"to":   in.Email,
	})

	// новый аккаунт: уведомление о регистрации админам
	s.notify(ctx, "user.was.registered", conv.Group{
		"lang":      in.LangCode,
		"userRealm": in.Realm,
		"userEmail": in.Email,
	})

	return userID, nil
}

// bindUserToRealm - привязывает существующего пользователя к realm. Повторная привязка тем же
// токеном (пользователь уже в realm) - не ошибка, а успешный переход к авторизации.
func (s *Service) bindUserToRealm(ctx context.Context, userID uuid.UUID, in dto.CreateUserOperation) error {
	err := s.insertUserRealm(ctx, userID, in)
	if err != nil {
		// повторное подтверждение (тот же токен): привязка уже существует - это успех
		if errors.Is(err, errUserAlreadyInRealm) {
			return nil
		}

		return err
	}

	// аккаунт уже существовал, создана новая привязка к realm (равносильно регистрации)
	s.notify(ctx, notice.KeyByEventAndRealm("user.registration.success", in.Realm), conv.Group{
		"lang": in.LangCode,
		"to":   in.Email,
	})

	return nil
}

// insertUserRealm - вставляет привязку пользователя к realm, переводя нарушение уникальности
// в errUserAlreadyInRealm (повторное подтверждение создания).
func (s *Service) insertUserRealm(ctx context.Context, userID uuid.UUID, in dto.CreateUserOperation) error {
	realmID, ok := s.realmRegistry.IDByName(in.Realm)
	if !ok {
		return errors.ErrInternalIncorrectInputData.WithDetails("realm is unknown", "realm", in.Realm)
	}

	err := s.storageUserRealm.Insert(
		ctx,
		entity.UserRealm{
			UserID:  userID,
			RealmID: realmID,
			Kind:    in.UserKind,
		},
	)
	if err != nil {
		if errors.Is(err, errors.ErrEventRecordAlreadyExists) {
			return errUserAlreadyInRealm
		}

		return s.errorWrapper.Wrap(err)
	}

	return nil
}

// notify - отправляет уведомление: сбой только логируется и не прерывает поток.
func (s *Service) notify(ctx context.Context, event string, args conv.Group) {
	if err := s.notifierAPI.Send(ctx, event, args); err != nil {
		s.logger.Error(ctx, "authuser: notice not sent", "event", event, "error", err)
	}
}
