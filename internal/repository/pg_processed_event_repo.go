package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.ProcessedEventRepo = (*PgProcessedEventRepository)(nil)

// PgProcessedEventRepository は PostgreSQL を使用した ProcessedEventRepo の実装です。
type PgProcessedEventRepository struct {
	pool *pgxpool.Pool
}

// NewPgProcessedEventRepository は PgProcessedEventRepository を生成します。
func NewPgProcessedEventRepository(pool *pgxpool.Pool) *PgProcessedEventRepository {
	return &PgProcessedEventRepository{pool: pool}
}

// Insert は processed_events 行を挿入します。新規挿入なら true を返します。
// event_id が既存の場合は ON CONFLICT DO NOTHING + RETURNING で false を返します。
func (r *PgProcessedEventRepository) Insert(ctx context.Context, eventID, eventType string) (bool, error) {
	var inserted string
	err := connFrom(ctx, r.pool).QueryRow(ctx,
		`INSERT INTO account.processed_events (event_id, event_type)
		 VALUES ($1, $2)
		 ON CONFLICT (event_id) DO NOTHING
		 RETURNING event_id`,
		eventID, eventType,
	).Scan(&inserted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("insert processed_events: %w", err)
	}
	return true, nil
}
