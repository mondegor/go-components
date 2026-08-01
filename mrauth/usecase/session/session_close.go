package session

import (
	"context"

	"github.com/mondegor/go-core/errors"
)

type (
	// CloseSession - закрытие сессии (logout) по refresh токену.
	CloseSession struct {
		tokenCloser  tokenCloser
		errorWrapper errors.Wrapper
	}

	tokenCloser interface {
		Close(ctx context.Context, refreshToken string) error
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

// Execute - отзывает все действующие токены сессии по её refresh токену
// (идемпотентно: пустой и неизвестный токен, как и уже закрытая сессия - это успех, а не ошибка).
func (uc *CloseSession) Execute(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}

	if err := uc.tokenCloser.Close(ctx, refreshToken); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
