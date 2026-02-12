package store

import "context"

type Store interface {
	Commit(ctx context.Context) error
	TransactionContext(ctx context.Context) (context.Context, error)
	Rollback(ctx context.Context) error
	WithAcquire(ctx context.Context) (dbCtx context.Context, err error)
	Release(ctx context.Context)
}

type store struct {
}