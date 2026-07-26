package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.BattleCountReversalRepo = (*BattleCountReversalRepository)(nil)

// BattleCountReversalRepository は port.BattleCountReversalRepo の PostgreSQL 実装。
type BattleCountReversalRepository struct {
	pool *pgxpool.Pool
}

// NewBattleCountReversalRepository は BattleCountReversalRepository を生成する。
func NewBattleCountReversalRepository(pool *pgxpool.Pool) *BattleCountReversalRepository {
	return &BattleCountReversalRepository{pool: pool}
}

// MarkReverted は account.battle_count_reversals 行を挿入する。新規挿入なら true、既存なら false。
func (r *BattleCountReversalRepository) MarkReverted(ctx context.Context, gameID string) (bool, error) {
	var inserted string
	err := connFrom(ctx, r.pool).QueryRow(ctx,
		`INSERT INTO account.battle_count_reversals (game_id)
		 VALUES ($1)
		 ON CONFLICT (game_id) DO NOTHING
		 RETURNING game_id`,
		gameID,
	).Scan(&inserted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("insert battle_count_reversals: %w", err)
	}
	return true, nil
}
