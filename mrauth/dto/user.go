package dto

import (
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserInRealm - сообщение для получателя.
	UserInRealm struct {
		Realm string
		Kind  string
		entity.User
	}

	// UserInfo - comment struct.
	UserInfo struct {
		User    entity.User
		Stat    entity.UserActivityStat
		Auth2fa entity.Auth2fa
		Realms  []entity.UserRealm
	}

	// UserActivityLog - сообщение для получателя.
	UserActivityLog = entity.UserActivityLog
)
