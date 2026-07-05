package repository

import (
	"errors"

	"github.com/google/uuid"
)

type (
	// TokenAlreadyRevokedError - ошибка, когда токен уже отозван.
	TokenAlreadyRevokedError struct {
		UserID    uuid.UUID
		SessionID uint32
	}
)

// ErrTokenExpired - token is expired.
var ErrTokenExpired = errors.New("token is expired")

// NewTokenAlreadyRevokedError - создаёт ошибку TokenAlreadyRevokedError для указанного типа параметра.
func NewTokenAlreadyRevokedError(userID uuid.UUID, sessionID uint32) error {
	return &TokenAlreadyRevokedError{
		UserID:    userID,
		SessionID: sessionID,
	}
}

func (e *TokenAlreadyRevokedError) Error() string {
	return "token is already revoked"
}
