package session

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
)

type (
	// CloseSession - закрытие сессии (logout) по refresh токену.
	CloseSession struct {
		tokenCloser  tokenCloser
		errorWrapper errors.Wrapper
	}

	tokenCloser interface {
		Close(ctx context.Context, userID uuid.UUID, refreshToken string) error
	}
)

// NewCloseSession - создаёт объект CloseSession.
func NewCloseSession(
	tokenCloser tokenCloser,
) *CloseSession {
	return &CloseSession{
		tokenCloser:  tokenCloser,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - отзывает все действующие токены сессии пользователя по её refresh токену
// (идемпотентно: пустой, неизвестный и чужой токен, как и уже закрытая сессия - это успех,
// а не ошибка; чужая сессия при этом не закрывается).
func (uc *CloseSession) Execute(ctx context.Context, userID uuid.UUID, refreshToken string) error {
	if userID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userID is empty")
	}

	if refreshToken == "" {
		return nil
	}

	if err := uc.tokenCloser.Close(ctx, userID, refreshToken); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
