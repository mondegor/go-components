package session

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrtype"
	"github.com/mondegor/go-sysmess/util/slices/ordered"

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
		appResolver      mrauth.AppResolver
		locationResolver mrauth.LocationResolver
		errorWrapper     errors.Wrapper
	}

	sessionLister interface {
		FetchListByUserID(ctx context.Context, userID uuid.UUID) ([]entity.Session, error)
	}

	openSessionFetcher interface {
		FetchOpenSessionIDs(ctx context.Context, userID uuid.UUID) (sessionIDs []uint32, err error)
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
	appResolver mrauth.AppResolver,
	locationResolver mrauth.LocationResolver,
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
		appResolver:      appResolver,
		locationResolver: locationResolver,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
	}
}

// GetList - возвращает список открытых сессий пользователя.
func (uc *List) GetList(ctx context.Context, userID uuid.UUID, currentAccessToken string) ([]dto.UserSession, error) {
	openSessionIDs, err := uc.openFetcher.FetchOpenSessionIDs(ctx, userID)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	if len(openSessionIDs) == 0 {
		return make([]dto.UserSession, 0), nil
	}

	var currentSessionID uint32

	if scopes, resolveErr := uc.resolver.FetchOneByAccessToken(ctx, currentAccessToken); resolveErr == nil {
		currentSessionID = scopes.SessionID
	}

	sessions, err := uc.storage.FetchListByUserID(ctx, userID)
	if err != nil {
		return nil, uc.errorWrapper.Wrap(err)
	}

	openSessionIDs = ordered.SortedUnique(openSessionIDs)

	list := make([]dto.UserSession, 0, len(openSessionIDs))

	for _, session := range sessions {
		if !ordered.BinaryContains(openSessionIDs, session.SessionID) {
			continue // сессия без действующих токенов является закрытой
		}

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
				IsCurrent:  session.SessionID == currentSessionID,
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
