package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
)

type Store struct {
	Pool    *pgxpool.Pool
	Queries *sqlcdb.Queries
}

func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{Pool: pool, Queries: sqlcdb.New(pool)}, nil
}

func (s *Store) Close() {
	if s.Pool != nil {
		s.Pool.Close()
	}
}

// WithTx runs fn inside a database transaction. Queries on the passed store use the tx.
func (s *Store) WithTx(ctx context.Context, fn func(q *sqlcdb.Queries) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.Queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// BeginTx starts a transaction and returns transactional queries.
func (s *Store) BeginTx(ctx context.Context) (pgx.Tx, *sqlcdb.Queries, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	return tx, s.Queries.WithTx(tx), nil
}
