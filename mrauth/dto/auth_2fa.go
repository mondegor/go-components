package dto

import (
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// User2FA - данные пользователя для подтверждения второго фактора.
	User2FA struct {
		ID        uuid.UUID
		Email     string
		Phone     uint64
		Action2FA secureoperation.ConfirmAction
	}
)
