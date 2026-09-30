package handler

import (
	"context"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
)

type (
	// operationRevoker - отзывает все незавершённые операции пользователя.
	operationRevoker interface {
		RevokeAll(ctx context.Context, actor dto.ActorMeta, reason logreason.Enum) error
	}
)
