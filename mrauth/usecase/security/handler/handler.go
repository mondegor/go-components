package handler

import (
	"context"

	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
)

type (
	// operationRevoker - отзывает все незавершённые операции пользователя.
	operationRevoker interface {
		RevokeAll(ctx context.Context, actor dto.ActorMeta, reason logreason.Enum) error
	}

	// securityLogStorage - хранилище журнала безопасности пользователя; запись выполняется
	// в транзакции вызывающего, поэтому событие фиксируется вместе с самой операцией.
	securityLogStorage interface {
		Insert(ctx context.Context, row entity.SecurityLogEvent) error
	}

	// actorPropsBuilder - дополняет props уведомления о событии безопасности контекстом клиента
	// (время события, IP, устройство).
	actorPropsBuilder interface {
		With(actor dto.ActorMeta, props conv.Group) conv.Group
	}
)
