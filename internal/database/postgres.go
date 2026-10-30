package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/config"
)

// DBTX is satisfied by both the connection pool and a transaction, so
// repository methods can run either standalone or inside a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func NewPool(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, err
	}
	poolCfg.MinConns = cfg.PoolMin
	poolCfg.MaxConns = cfg.PoolMax
	poolCfg.MaxConnIdleTime = 30 * time.Second
	poolCfg.ConnConfig.ConnectTimeout = 5 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Transactor runs functions within a database transaction.
type Transactor struct {
	pool *pgxpool.Pool
}

func NewTransactor(pool *pgxpool.Pool) *Transactor { return &Transactor{pool: pool} }

// InTx commits when fn succeeds and rolls back when it returns an error.
func (t *Transactor) InTx(ctx context.Context, fn func(DBTX) error) error {
	return pgx.BeginFunc(ctx, t.pool, func(tx pgx.Tx) error { return fn(tx) })
}
