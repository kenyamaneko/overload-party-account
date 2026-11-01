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

// ProcessedEventRepository は処理済みイベントの記録の PostgreSQL 実装である。
type ProcessedEventRepository struct {
	pool *pgxpool.Pool
}

// NewProcessedEventRepository は ProcessedEventRepository を生成する。
func NewProcessedEventRepository(pool *pgxpool.Pool) *ProcessedEventRepository {
	return &ProcessedEventRepository{pool: pool}
}

// InsertProcessedEvent はイベントIDを処理済みとして記録する。新しく記録できたときは true、記録済みのときは false を返す。
func (r *ProcessedEventRepository) InsertProcessedEvent(ctx context.Context, eventID, eventType string) (bool, error) {
	var insertedEventID string
	err := resolveDBTX(ctx, r.pool).QueryRow(ctx,
		`INSERT INTO account.processed_events (event_id, event_type)
		 VALUES ($1, $2)
		 ON CONFLICT (event_id) DO NOTHING
		 RETURNING event_id`,
		eventID, eventType,
	).Scan(&insertedEventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("insert processed_events: %w", err)
	}
	return true, nil
}
