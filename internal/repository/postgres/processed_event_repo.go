package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.ProcessedEventRepo = (*ProcessedEventRepository)(nil)

// ProcessedEventRepository は port.ProcessedEventRepo の PostgreSQL 実装。
type ProcessedEventRepository struct {
	pool *pgxpool.Pool
}

// NewProcessedEventRepository は ProcessedEventRepository を生成する。
func NewProcessedEventRepository(pool *pgxpool.Pool) *ProcessedEventRepository {
	return &ProcessedEventRepository{pool: pool}
}

// Insert は account.processed_events 行を挿入する。新規挿入なら true、既存なら false。
func (r *ProcessedEventRepository) Insert(ctx context.Context, eventID, eventType string) (bool, error) {
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
