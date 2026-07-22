package session

import (
	"context"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
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
		errorWrapper: errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - отзывает все действующие токены сессии по её refresh токену.
func (uc *CloseSession) Execute(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return errors.ErrIncorrectInputData.New("refreshToken is empty")
	}

	if err := uc.tokenCloser.Close(ctx, refreshToken); err != nil {
		// отзывать нечего - токен не найден либо сессия уже отозвана
		if errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
			return mrauth.ErrTokenInvalid
		}

		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
