package dto

import (
	"github.com/google/uuid"
)

type (
	// User2FA - информация о пользователе для 2FA.
	User2FA struct {
		ID        uuid.UUID
		Email     string
		Phone     uint64
		Action2FA ConfirmAction
	}

	// Auth2faCancel - comment struct.
	Auth2faCancel struct {
		Secret string
	}
)
