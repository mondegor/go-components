package session

import (
	"context"

	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth"
)

type (
	// CloseSession - comment struct.
	CloseSession struct {
		tokenCloser  tokenCloser
		errorWrapper mrerr.UseCaseErrorWrapper
	}

	tokenCloser interface {
		Close(ctx context.Context, accessToken string) error
	}
)

// NewCloseSession - создаёт объект CloseSession.
func NewCloseSession(
	tokenCloser tokenCloser,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *CloseSession {
	return &CloseSession{
		tokenCloser:  tokenCloser,
		errorWrapper: mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.CloseSession"),
	}
}

// Execute - comments method.
func (uc *CloseSession) Execute(ctx context.Context, accessToken string) error {
	if accessToken == "" {
		return mr.ErrUseCaseIncorrectInputData.New("accessToken is empty")
	}

	// :TODO можно закрывать сессию по refresh token при jwt, иначе сейчас генерируется ошибка 404

	if err := uc.tokenCloser.Close(ctx, accessToken); err != nil {
		if uc.errorWrapper.IsNotFoundError(err) {
			return mrauth.ErrTokenNotFoundOrExpired.Wrap(mr.ErrUseCaseEntityNotFound)
		}

		return uc.errorWrapper.WrapErrorFailed(err)
	}

	return nil
}
