package core

import (
	"context"
)

//go:generate mockgen -source=trace_manager.go -destination=./mock/trace_manager.go

type (
	// TraceManager - отвечает за установку ID процессов в контекст и за доступ к ним используемых в трейсинге.
	TraceManager interface {
		WithCorrelationID(ctx context.Context, id string) context.Context
		WithGeneratedRequestID(ctx context.Context) context.Context
		RequestID(ctx context.Context) string
		WithGeneratedProcessID(ctx context.Context) context.Context
		WithGeneratedWorkerID(ctx context.Context) context.Context
		WithGeneratedTaskID(ctx context.Context) context.Context
		NewContextWithIDs(originalCtx context.Context) context.Context
	}
)
