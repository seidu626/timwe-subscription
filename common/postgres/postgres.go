package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PGXStore handles database communication with PostgresSQL.
type PGXStore struct {
	Logger *zap.Logger
	// Postgres database.PGX
	Postgres *pgxpool.Pool
}

func NewPGXStore(logger *zap.Logger, pool *pgxpool.Pool) *PGXStore {
	return &PGXStore{Logger: logger, Postgres: pool}
}

// TransactionContext returns a copy of the parent context which begins a transaction
// to PostgresSQL.
//
// Once the transaction is over, you must call db.Commit(ctx) to make the changes effective.
// This might live in the go-pkg/postgres package later for the sake of code reuse.
func (db *PGXStore) TransactionContext(ctx context.Context) (context.Context, error) {
	tx, err := db.Conn(ctx).Begin(ctx)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, txCtx{}, tx), nil
}

// Commit transaction from context.
func (db *PGXStore) Commit(ctx context.Context) error {
	if tx, ok := ctx.Value(txCtx{}).(pgx.Tx); ok && tx != nil {
		return tx.Commit(ctx)
	}
	return errors.New("context has no transaction")
}

// Rollback transaction from context.
func (db *PGXStore) Rollback(ctx context.Context) error {
	if tx, ok := ctx.Value(txCtx{}).(pgx.Tx); ok && tx != nil {
		return tx.Rollback(ctx)
	}
	return errors.New("context has no transaction")
}

// WithAcquire returns a copy of the parent context which acquires a connection
// to PostgreSQL from pgxpool to make sure commands executed in series reuse the
// same database connection.
//
// To release the connection back to the pool, you must call postgres.Release(ctx).
//
// Example:
// dbCtx := db.WithAcquire(ctx)
// defer postgres.Release(dbCtx)
func (db *PGXStore) WithAcquire(ctx context.Context) (dbCtx context.Context, err error) {
	if _, ok := ctx.Value(connCtx{}).(*pgxpool.Conn); ok {
		panic("context already has a connection acquired")
	}
	res, err := db.Postgres.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, connCtx{}, res), nil
}

// Release PostgreSQL connection acquired by context back to the pool.
func (db *PGXStore) Release(ctx context.Context) {
	if res, ok := ctx.Value(connCtx{}).(*pgxpool.Conn); ok && res != nil {
		res.Release()
	}
}

// txCtx key.
type txCtx struct{}

// connCtx key.
type connCtx struct{}

// Conn returns a PostgreSQL transaction if one exists.
// If not, returns a connection if a connection has been acquired by calling WithAcquire.
// Otherwise, it returns *pgxpool.Pool which acquires the connection and closes it immediately after a SQL command is executed.
func (db *PGXStore) Conn(ctx context.Context) PGXQuerier {
	if tx, ok := ctx.Value(txCtx{}).(pgx.Tx); ok && tx != nil {
		return tx
	}
	if res, ok := ctx.Value(connCtx{}).(*pgxpool.Conn); ok && res != nil {
		return res
	}
	return db.Postgres
}
