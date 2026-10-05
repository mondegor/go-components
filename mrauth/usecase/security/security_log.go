package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// GetSecurityLog - получение журнала безопасности пользователя постранично,
	// от самых свежих событий к старым.
	GetSecurityLog struct {
		storage          securityLogFetcher
		appResolver      mrauth.AppResolver
		locationResolver mrauth.LocationResolver
		errorWrapper     errors.Wrapper
	}

	securityLogFetcher interface {
		FetchByUserID(
			ctx context.Context,
			userID uuid.UUID,
			cursor mrstorage.IDCursor,
		) (rows []entity.SecurityLogEvent, hasNext bool, err error)
	}
)

// NewGetSecurityLog - создаёт объект GetSecurityLog.
func NewGetSecurityLog(
	storage securityLogFetcher,
	appResolver mrauth.AppResolver, // OPTIONAL
	locationResolver mrauth.LocationResolver, // OPTIONAL
) *GetSecurityLog {
	if appResolver == nil {
		appResolver = mrauth.DefaultAppResolver
	}

	if locationResolver == nil {
		locationResolver = mrauth.DefaultLocationResolver
	}

	return &GetSecurityLog{
		storage:          storage,
		appResolver:      appResolver,
		locationResolver: locationResolver,
		errorWrapper:     errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - возвращает страницу журнала безопасности пользователя, начиная с позиции cursor,
// и признак наличия записей за ней. Приложение и устройство определяются по user agent записи,
// местоположение - по её IP.
func (uc *GetSecurityLog) Execute(
	ctx context.Context,
	userID uuid.UUID,
	cursor mrstorage.IDCursor,
) (items []dto.SecurityLogItem, hasNext bool, err error) {
	if userID == uuid.Nil {
		return nil, false, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	rows, hasNext, err := uc.storage.FetchByUserID(ctx, userID, cursor)
	if err != nil {
		return nil, false, uc.errorWrapper.Wrap(err)
	}

	items = make([]dto.SecurityLogItem, 0, len(rows))

	for _, row := range rows {
		appName, deviceName := uc.appResolver(row.UserAgent)

		item := dto.SecurityLogItem{
			RecordID:   row.RecordID,
			EventType:  row.EventType,
			AppName:    appName,
			DeviceName: deviceName,
			IP:         uc.locationResolver(row.ClientIP.Real, mrauth.LocationOnlyIP),
			Location:   uc.locationResolver(row.ClientIP.Real, mrauth.LocationOrEmpty),
			CreatedAt:  row.CreatedAt,
		}

		if row.Extra != nil {
			item.OldValue = row.Extra.OldValue
			item.NewValue = row.Extra.NewValue
			item.Factor = row.Extra.Factor
			item.Remaining = row.Extra.Remaining
		}

		items = append(items, item)
	}

	return items, hasNext, nil
}
