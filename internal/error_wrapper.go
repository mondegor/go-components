package core

import "github.com/mondegor/go-sysmess/mrerr/mr"

type (
	// ErrorWrapper - помощник для оборачивания ошибок.
	ErrorWrapper interface {
		WrapError(err error, attrs ...any) error
	}

	// UseCaseErrorWrapper - помощник для оборачивания UseCase ошибок.
	UseCaseErrorWrapper interface {
		IsNotFoundOrNotAffectedError(err error) bool
		WrapErrorFailed(err error, attrs ...any) error
		WrapErrorNotFoundOrFailed(err error, attrs ...any) error
	}
)

// NewStorageErrorWrapper - создаёт объект StorageErrorWrapper.
func NewStorageErrorWrapper(source string) *mr.StorageErrorWrapper {
	return mr.NewStorageErrorWrapper(source)
}

// NewUseCaseErrorWrapper - создаёт объект UseCaseErrorWrapper.
func NewUseCaseErrorWrapper(source string) *mr.UseCaseErrorWrapper {
	return mr.NewUseCaseErrorWrapper(source)
}
