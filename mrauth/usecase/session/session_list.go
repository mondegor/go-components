package session

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrtype"

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
)

// NewList - создаёт объект List.
func NewList(
	storage sessionLister,
	openFetcher openSessionFetcher,
	closer sessionCloser,
	resolver sessionResolver,
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
		realmRegistry:    realmRegistry,
		appResolver:      appResolver,
		locationResolver: locationResolver,
		limiter:          newLimitResolver(allowedRealms),
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
	}
}

// GetList - возвращает список открытых сессий пользователя.
func (uc *List) GetList(ctx context.Context, userID uuid.UUID, currentAccessToken string) ([]dto.UserSession, error) {
	scopes, err := uc.resolver.FetchOneByAccessToken(ctx, currentAccessToken)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	realmID, ok := uc.realmRegistry.IDByName(scopes.Realm)
	if !ok {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("realm is unknown", "realm", scopes.Realm)
	}

	// список скоупится по realm текущего токена: лимит сессий per-(user, realm), поэтому
	// и показываем только сессии этого realm, согласованно с обрезкой по его лимиту ниже
	openSessionIDs, err := uc.openFetcher.FetchOpenSessionIDs(ctx, userID, realmID)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	// текущая сессия обязана присутствовать среди открытых сессий своего realm (её токен только что
	// успешно зарезолвлен) - иначе нарушен инвариант; пустой набор открытых сессий тоже аномалия.
	if !slices.Contains(openSessionIDs, scopes.SessionID) {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails(
			"current session is not among open sessions", "sessionId", scopes.SessionID,
		)
	}

	// Метаданные выбираются ровно по открытым сессиям realm (отдельная фильтрация по членству не
	// нужна), уже упорядоченные по активности и обрезанные по лимиту: пользователь может временно
	// держать больше сессий, чем лимит (фоновая чистка ещё не сработала) - показываем только самые
	// активные в рамках лимита.
	sessions, err := uc.storage.FetchOrderedListByUserIDAndSessionIDs(
		ctx, userID, openSessionIDs, uc.limiter.Limit(realmID, scopes.Kind),
	)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	// текущая сессия могла выпасть за пределы лимита (как наименее активная из открытых) - тогда её
	// нет в усечённом списке. Догружаем её отдельным запросом и заменяем последнюю (наименее активную)
	// строку, сохраняя размер списка в рамках лимита и гарантируя IsCurrent для текущей сессии.
	if !slices.ContainsFunc(sessions, func(s entity.Session) bool { return s.SessionID == scopes.SessionID }) {
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
