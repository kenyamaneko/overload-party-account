package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.PlayerViewRepo = (*PlayerViewRepository)(nil)

// PlayerViewRepository は port.PlayerViewRepo の PostgreSQL 実装。
// players + player_progression + player_factions(is_initial=TRUE) を JOIN で結合した Read Model を返す。
type PlayerViewRepository struct {
	pool *pgxpool.Pool
}

// NewPlayerViewRepository は PlayerViewRepository を生成する。
func NewPlayerViewRepository(pool *pgxpool.Pool) *PlayerViewRepository {
	return &PlayerViewRepository{pool: pool}
}

// FindByID は player_id で Read Model を返す。該当なしは port.ErrNotFound。
func (r *PlayerViewRepository) FindByID(ctx context.Context, playerID string) (*domain.PlayerView, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx, playerViewSelectByID, playerID)
	v, err := scanPlayerView(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("find player view by id: %w", err)
	}
	return v, nil
}

// FindByFirebaseUID は firebase_uid で Read Model を返す。該当なしは port.ErrNotFound。
func (r *PlayerViewRepository) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.PlayerView, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx, playerViewSelectByFirebaseUID, firebaseUID)
	v, err := scanPlayerView(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, port.ErrNotFound
		}
		return nil, fmt.Errorf("find player view by firebase_uid: %w", err)
	}
	return v, nil
}

const playerViewSelectColumns = `p.player_id, p.firebase_uid, p.name, p.is_premium, p.equipped_icon_no,
	        p.onboarding_status, p.premium_expires_at, p.created_at, p.updated_at,
	        pp.level, pp.exp, pf.faction`

const playerViewSelectByID = `SELECT ` + playerViewSelectColumns + `
		   FROM account.players p
		   JOIN account.player_progression pp ON p.player_id = pp.player_id
		   LEFT JOIN account.player_factions pf ON pf.player_id = p.player_id AND pf.is_initial
		  WHERE p.player_id = $1`

const playerViewSelectByFirebaseUID = `SELECT ` + playerViewSelectColumns + `
		   FROM account.players p
		   JOIN account.player_progression pp ON p.player_id = pp.player_id
		   LEFT JOIN account.player_factions pf ON pf.player_id = p.player_id AND pf.is_initial
		  WHERE p.firebase_uid = $1 LIMIT 1`

func scanPlayerView(row pgx.Row) (*domain.PlayerView, error) {
	var v domain.PlayerView
	err := row.Scan(
		&v.Player.PlayerID,
		&v.Player.FirebaseUID,
		&v.Player.Name,
		&v.Player.IsPremium,
		&v.Player.EquippedIconNo,
		&v.Player.OnboardingStatus,
		&v.Player.PremiumExpiresAt,
		&v.Player.CreatedAt,
		&v.Player.UpdatedAt,
		&v.Level,
		&v.Exp,
		&v.InitialFaction,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
