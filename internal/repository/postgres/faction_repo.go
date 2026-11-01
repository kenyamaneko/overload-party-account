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

// FactionRepository はプレイヤーの所持陣営の永続化の PostgreSQL 実装である。
type FactionRepository struct {
	pool *pgxpool.Pool
}

// NewFactionRepository は FactionRepository を生成する。
func NewFactionRepository(pool *pgxpool.Pool) *FactionRepository {
	return &FactionRepository{pool: pool}
}

// AddPlayerFaction は陣営を所持陣営に加える。初期陣営にはしない。所持済みのときは何も変えない。
// プレイヤーが登録されていなければ port.ErrNotFound を返す。
func (r *FactionRepository) AddPlayerFaction(ctx context.Context, playerID, faction string) error {
	_, err := resolveDBTX(ctx, r.pool).Exec(ctx,
		`INSERT INTO account.player_factions (player_id, faction, is_initial)
		 VALUES ($1, $2, FALSE)
		 ON CONFLICT (player_id, faction) DO NOTHING`,
		playerID, faction,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("insert player faction: player %s: %w: %w", playerID, port.ErrNotFound, err)
		}
		return fmt.Errorf("insert player faction: %w", err)
	}
	return nil
}

// SetInitialFaction は陣営を初期陣営にし、所持陣営にも加える。
// 初期陣営がすでにあるときは、同じ陣営でも port.ErrAlreadyExists を返す。
func (r *FactionRepository) SetInitialFaction(ctx context.Context, playerID, faction string) error {
	commandTag, err := resolveDBTX(ctx, r.pool).Exec(ctx,
		`INSERT INTO account.player_factions (player_id, faction, is_initial)
		 VALUES ($1, $2, TRUE)
		 ON CONFLICT (player_id, faction) DO UPDATE SET is_initial = TRUE
		 WHERE NOT account.player_factions.is_initial`,
		playerID, faction,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("set initial faction: %w: %w", port.ErrAlreadyExists, err)
		}
		return fmt.Errorf("set initial faction: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("set initial faction: faction %s of player %s is already initial: %w", faction, playerID, port.ErrAlreadyExists)
	}
	return nil
}

// GetPlayerFactions は所持陣営の一覧を返す。順序は決まっていない。所持陣営がなければ空の一覧を返す。
func (r *FactionRepository) GetPlayerFactions(ctx context.Context, playerID string) ([]string, error) {
	rows, err := resolveDBTX(ctx, r.pool).Query(ctx,
		`SELECT faction FROM account.player_factions WHERE player_id = $1`,
		playerID)
	if err != nil {
		return nil, fmt.Errorf("query player factions: %w", err)
	}
	defer rows.Close()

	factions := []string{}
	for rows.Next() {
		var faction string
		if err := rows.Scan(&faction); err != nil {
			return nil, fmt.Errorf("scan faction: %w", err)
		}
		factions = append(factions, faction)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate factions: %w", err)
	}
	return factions, nil
}

// GetInitialFaction は初期陣営を返す。選択されていなければ nil を返す。
func (r *FactionRepository) GetInitialFaction(ctx context.Context, playerID string) (*string, error) {
	var faction string
	err := resolveDBTX(ctx, r.pool).QueryRow(ctx,
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
