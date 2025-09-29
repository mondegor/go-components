package dto

import (
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// User2FA - сообщение для получателя.
	User2FA struct {
		ID        uuid.UUID
		Email     string
		Phone     uint64
		Action2FA entity.ConfirmAction
	}
)
