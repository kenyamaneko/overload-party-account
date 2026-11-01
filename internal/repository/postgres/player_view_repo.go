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

// PlayerViewRepository はプレイヤーの参照用の情報の PostgreSQL 実装である。
type PlayerViewRepository struct {
	pool *pgxpool.Pool
}

// NewPlayerViewRepository は PlayerViewRepository を生成する。
func NewPlayerViewRepository(pool *pgxpool.Pool) *PlayerViewRepository {
	return &PlayerViewRepository{pool: pool}
}

// GetByID はプレイヤーの参照用の情報を返す。登録されていなければ port.ErrNotFound を返す。
func (r *PlayerViewRepository) GetByID(ctx context.Context, playerID string) (*domain.PlayerView, error) {
	row := resolveDBTX(ctx, r.pool).QueryRow(ctx, playerViewSelectByID, playerID)
	v, err := scanPlayerView(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("get player view by id: %w", err)
	}
	return v, nil
}

// GetByFirebaseUID は FirebaseUID に対応するプレイヤーの参照用の情報を返す。登録されていなければ port.ErrNotFound を返す。
func (r *PlayerViewRepository) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.PlayerView, error) {
	row := resolveDBTX(ctx, r.pool).QueryRow(ctx, playerViewSelectByFirebaseUID, firebaseUID)
	v, err := scanPlayerView(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, port.ErrNotFound
		}
		return nil, fmt.Errorf("get player view by firebase_uid: %w", err)
	}
	return v, nil
}

const playerViewSelectColumns = `p.player_id, p.firebase_uid, p.name, p.is_premium, p.equipped_icon_no,
	        p.onboarding_status, p.premium_expires_at, p.created_at, p.updated_at,
	        pp.exp, pf.faction`

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
		&v.Exp,
		&v.InitialFaction,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
