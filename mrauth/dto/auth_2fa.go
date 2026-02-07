package dto

import (
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// User2FA - информация о пользователе для 2FA.
	User2FA struct {
		ID        uuid.UUID
		Email     string
		Phone     uint64
		Action2FA secureoperation.ConfirmAction
	}

	// Auth2faCancel - comment struct.
	Auth2faCancel struct {
		Secret string
	}
)
