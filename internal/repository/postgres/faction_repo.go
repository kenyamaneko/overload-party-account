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

// FactionRepository は port.FactionRepo の PostgreSQL 実装。
type FactionRepository struct {
	pool *pgxpool.Pool
}

// NewFactionRepository は FactionRepository を生成する。
func NewFactionRepository(pool *pgxpool.Pool) *FactionRepository {
	return &FactionRepository{pool: pool}
}

// AddPlayerFaction は player_factions に is_initial=FALSE で 1 行追加する。
// (player_id, faction) PK の ON CONFLICT DO NOTHING で冪等。is_initial の昇格は行わない。
func (r *FactionRepository) AddPlayerFaction(ctx context.Context, playerID, faction string) error {
	_, err := connFrom(ctx, r.pool).Exec(ctx,
		`INSERT INTO account.player_factions (player_id, faction, is_initial)
		 VALUES ($1, $2, FALSE)
		 ON CONFLICT (player_id, faction) DO NOTHING`,
		playerID, faction,
	)
	if err != nil {
		return fmt.Errorf("insert player faction: %w", err)
	}
	return nil
}

// SetInitialFaction は (player_id, faction) を is_initial=TRUE で INSERT する。
// PK 重複や partial unique index 違反は DB エラーとしてそのまま返す。
func (r *FactionRepository) SetInitialFaction(ctx context.Context, playerID, faction string) error {
	_, err := connFrom(ctx, r.pool).Exec(ctx,
		`INSERT INTO account.player_factions (player_id, faction, is_initial)
		 VALUES ($1, $2, TRUE)`,
		playerID, faction,
	)
	if err != nil {
		return fmt.Errorf("set initial faction: %w", err)
	}
	return nil
}

// GetPlayerFactions はプレイヤーの所持ファクション一覧を返す (is_initial 区別なし)。
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

// GetInitialFaction はプレイヤーの initial faction を返す。未選択なら (nil, nil)。
func (r *FactionRepository) GetInitialFaction(ctx context.Context, playerID string) (*string, error) {
	var faction string
	err := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT faction FROM account.player_factions
		 WHERE player_id = $1 AND is_initial = TRUE`,
		playerID,
	).Scan(&faction)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get initial faction: %w", err)
	}
	return &faction, nil
}
