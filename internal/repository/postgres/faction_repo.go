package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.FactionRepo = (*FactionRepository)(nil)

// FactionRepository は PostgreSQL を使用した FactionRepo の実装である。
type FactionRepository struct {
	pool *pgxpool.Pool
}

// NewFactionRepository は FactionRepository を生成する。
func NewFactionRepository(pool *pgxpool.Pool) *FactionRepository {
	return &FactionRepository{pool: pool}
}

// AddPlayerFaction はプレイヤーファクションを追加する（ON CONFLICT DO NOTHING）。
func (r *FactionRepository) AddPlayerFaction(ctx context.Context, playerID, faction, source string) error {
	_, err := connFrom(ctx, r.pool).Exec(ctx,
		`INSERT INTO account.player_factions (player_id, faction, source)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (player_id, faction) DO NOTHING`,
		playerID, faction, source,
	)
	if err != nil {
		return fmt.Errorf("insert player faction: %w", err)
	}
	return nil
}

// InsertInitial は player_factions 行を挿入し、RETURNING で新規行かを判定する。
// 複合 PK (player_id, faction) が一意性を保証するため、
// ON CONFLICT DO NOTHING RETURNING で既存判定を SELECT なしで実現する。
func (r *FactionRepository) InsertInitial(ctx context.Context, playerID, faction, source string) (bool, error) {
	var inserted string
	err := connFrom(ctx, r.pool).QueryRow(ctx,
		`INSERT INTO account.player_factions (player_id, faction, source)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (player_id, faction) DO NOTHING
		 RETURNING player_id`,
		playerID, faction, source,
	).Scan(&inserted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("insert initial player faction: %w", err)
	}
	return true, nil
}

// GetPlayerFactions はプレイヤーの所持ファクション一覧を返す。
func (r *FactionRepository) GetPlayerFactions(ctx context.Context, playerID string) ([]string, error) {
	rows, err := connFrom(ctx, r.pool).Query(ctx,
		`SELECT faction FROM account.player_factions WHERE player_id = $1`,
		playerID)
	if err != nil {
		return nil, fmt.Errorf("query player factions: %w", err)
	}
	defer rows.Close()

	var factions []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, fmt.Errorf("scan faction: %w", err)
		}
		factions = append(factions, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate factions: %w", err)
	}
	return factions, nil
}
