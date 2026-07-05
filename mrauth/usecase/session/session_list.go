package session

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrtype"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// List - получение списка открытых сессий пользователя и их закрытие.
	List struct {
		storage          sessionLister
		openFetcher      openSessionFetcher
		closer           sessionCloser
		resolver         sessionResolver
		userRealmFetcher userRealmFetcher
		realmRegistry    mrauth.RealmRegistry
		appResolver      mrauth.AppResolver
		locationResolver mrauth.LocationResolver
		limiter          *limitResolver
		errorWrapper     errors.Wrapper
	}

	sessionLister interface {
		FetchOrderedListByUserIDAndSessionIDs(ctx context.Context, userID uuid.UUID, sessionIDs []uint32, limit int) ([]entity.Session, error)
	}

	openSessionFetcher interface {
		FetchOpenSessionIDs(ctx context.Context, userID uuid.UUID, realmID uint16) (sessionIDs []uint32, err error)
	}

	sessionCloser interface {
		RevokeTokensBySessionIDs(ctx context.Context, userID uuid.UUID, sessionIDs []uint32) error
	}

	sessionResolver interface {
		FetchOneByAccessToken(ctx context.Context, accessToken string) (dto.UserScopes, error)
	}

	userRealmFetcher interface {
		FetchOne(ctx context.Context, userID uuid.UUID, realmID uint16) (entity.UserRealm, error)
	}
)

// NewList - создаёт объект List.
func NewList(
	storage sessionLister,
	openFetcher openSessionFetcher,
	closer sessionCloser,
	resolver sessionResolver,
	userRealmFetcher userRealmFetcher,
	realmRegistry mrauth.RealmRegistry,
	appResolver mrauth.AppResolver,
	locationResolver mrauth.LocationResolver,
	allowedRealms []LimitRealm,
) *List {
	if appResolver == nil {
		appResolver = func(_ string) (string, string) {
			return "", ""
		}
	}

	if locationResolver == nil {
		locationResolver = func(ip string) string {
			return ip
		}
	}

	return &List{
		storage:          storage,
		openFetcher:      openFetcher,
		closer:           closer,
		resolver:         resolver,
		userRealmFetcher: userRealmFetcher,
		realmRegistry:    realmRegistry,
		appResolver:      appResolver,
		locationResolver: locationResolver,
		limiter:          newLimitResolver(allowedRealms),
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
	}
}

// GetList - возвращает список открытых сессий пользователя. Если realm не задан, берётся realm
// текущей сессии; иначе - указанный realm при условии членства в нём пользователя.
func (uc *List) GetList(ctx context.Context, userID uuid.UUID, currentAccessToken, realm string) ([]dto.UserSession, error) {
	scopes, err := uc.resolver.FetchOneByAccessToken(ctx, currentAccessToken)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	// realm текущей сессии - realm по умолчанию; тогда работает полная логика с текущей сессией.
	isCurrentRealm := realm == "" || realm == scopes.Realm

	realmName := scopes.Realm
	if !isCurrentRealm {
		realmName = realm
	}

	realmID, ok := uc.realmRegistry.IDByName(realmName)
	if !ok {
		// realm текущей сессии обязан быть известен (нарушение инварианта); чужой realm задаёт клиент
		if isCurrentRealm {
			return nil, errors.ErrInternalIncorrectInputData.WithDetails("realm is unknown", "realm", realmName)
		}

		return nil, errors.ErrIncorrectInputData.New("realm is unknown")
	}

	// лимит сессий задаётся per-(realm, kind), а kind пользователя зависит от realm: для чужого realm
	// проверяем членство и берём его kind (отсутствие привязки - ошибка "не найдено")
	kind := scopes.Kind

	if !isCurrentRealm {
		userRealm, err := uc.userRealmFetcher.FetchOne(ctx, userID, realmID)
		if err != nil {
			// нет привязки к запрошенному realm - клиент не имеет к нему доступа (403), а не 404/500
			if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
				return nil, errors.ErrAccessForbidden
			}

			return nil, uc.errorWrapper.Wrap(err)
		}

		kind = userRealm.Kind
	}

	// список формируется по realm: лимит сессий per-(user, realm), поэтому показываем только сессии
	// этого realm, согласованно с обрезкой по его лимиту ниже
	openSessionIDs, err := uc.openFetcher.FetchOpenSessionIDs(ctx, userID, realmID)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	// инвариант "текущая сессия среди открытых" и догрузка текущей сессии ниже валидны только для
	// realm текущей сессии: в чужом realm текущей сессии нет по определению, а список может быть пуст
	if isCurrentRealm && !slices.Contains(openSessionIDs, scopes.SessionID) {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails(
			"current session is not among open sessions", "sessionId", scopes.SessionID,
		)
	}

	// метаданные выбираются ровно по открытым сессиям realm (отдельная фильтрация по членству не нужна),
	// уже упорядоченные по активности и обрезанные по лимиту: пользователь может временно держать
	// больше сессий, чем лимит (фоновая чистка ещё не сработала) - показываем только самые активные в рамках лимита
	sessions, err := uc.storage.FetchOrderedListByUserIDAndSessionIDs(
		ctx, userID, openSessionIDs, uc.limiter.Limit(realmID, kind),
	)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	// текущая сессия могла выпасть за пределы лимита (как наименее активная из открытых) - тогда её
	// нет в усечённом списке. Догружаем её отдельным запросом и заменяем последнюю (наименее активную)
	// строку, сохраняя размер списка в рамках лимита и гарантируя IsCurrent для текущей сессии
	if isCurrentRealm && !slices.ContainsFunc(sessions, func(s entity.Session) bool { return s.SessionID == scopes.SessionID }) {
		current, err := uc.storage.FetchOrderedListByUserIDAndSessionIDs(ctx, userID, []uint32{scopes.SessionID}, 0)
		if err != nil {
			return nil, uc.errorWrapper.Wrap(err)
		}

		if len(current) == 0 {
			return nil, errors.ErrInternalIncorrectInputData.WithDetails(
				"current session metadata not found", "sessionId", scopes.SessionID,
			)
		}

		sessions[len(sessions)-1] = current[0]
	}

	list := make([]dto.UserSession, 0, len(sessions))

	for _, session := range sessions {
		appName, deviceName := uc.appResolver(session.UserAgent)
		lastIP := mrtype.NewIP(session.LastIP).String()

		list = append(
			list,
			dto.UserSession{
				SessionID:  session.SessionID,
				AppName:    appName,
				DeviceName: deviceName,
				LastIP:     lastIP,
				Location:   uc.locationResolver(lastIP),
				CreatedAt:  session.CreatedAt,
				UpdatedAt:  session.UpdatedAt,
				IsCurrent:  session.SessionID == scopes.SessionID,
			},
		)
	}

	return list, nil
}

// Close - закрывает указанные сессии пользователя (идемпотентно: чужие/несуществующие игнорируются).
func (uc *List) Close(ctx context.Context, userID uuid.UUID, sessionIDs []uint32) error {
	if len(sessionIDs) == 0 {
		return errors.ErrIncorrectInputData.New("sessionIDs is empty")
	}

	if err := uc.closer.RevokeTokensBySessionIDs(ctx, userID, sessionIDs); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
