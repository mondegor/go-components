package notify

import (
	"context"
	"maps"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserEmailNotifier - декоратор Notifier: получателя, заданного ID пользователя,
	// заменяет на email этого пользователя.
	UserEmailNotifier struct {
		notifierAPI  mrauth.Notifier
		storageUser  userFetcher
		errorWrapper errors.Wrapper
	}

	userFetcher interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (entity.User, error)
	}
)

// NewUserEmailNotifier - создаёт объект UserEmailNotifier.
func NewUserEmailNotifier(notifierAPI mrauth.Notifier, storageUser userFetcher) *UserEmailNotifier {
	return &UserEmailNotifier{
		notifierAPI:  notifierAPI,
		storageUser:  storageUser,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Send - отправляет уведомление: если props["to"] содержит ID пользователя (uuid.UUID),
// то подставляет вместо него email пользователя, иначе передаёт props без изменений.
func (n *UserEmailNotifier) Send(ctx context.Context, key string, props map[string]any) error {
	userID, ok := props["to"].(uuid.UUID)
	if !ok {
		return n.notifierAPI.Send(ctx, key, props)
	}

	user, err := n.storageUser.FetchOne(ctx, userID)
	if err != nil {
		return n.errorWrapper.Wrap(err, "userId", userID)
	}

	resolved := maps.Clone(props) // props вызывающего не изменяются
	resolved["to"] = user.Email

	return n.notifierAPI.Send(ctx, key, resolved)
}
