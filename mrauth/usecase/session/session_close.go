package session

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
)

type (
	// CloseSession - comment struct.
	CloseSession struct {
		tokenCloser  tokenCloser
		errorWrapper errors.Wrapper
	}

	tokenCloser interface {
		Close(ctx context.Context, accessToken string) error
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

// Execute - comments method.
func (uc *CloseSession) Execute(ctx context.Context, accessToken string) error {
	if accessToken == "" {
		return errors.ErrIncorrectInputData.New("accessToken is empty")
	}

	// :TODO можно закрывать сессию по refresh token при jwt, иначе сейчас генерируется ошибка 404

	if err := uc.tokenCloser.Close(ctx, accessToken); err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return mrauth.ErrTokenNotFoundOrExpired
		}

		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
