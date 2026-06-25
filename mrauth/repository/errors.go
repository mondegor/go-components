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

var (
	// ErrTokenExpired - token is expired.
	ErrTokenExpired = errors.New("token is expired")

	// ErrSessionIDCollision - такой session_id уже занят (конфликт первичного ключа сессии).
	ErrSessionIDCollision = errors.New("session id collision")
)

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
