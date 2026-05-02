package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.PlayerSettingsRepo = (*PlayerSettingsRepository)(nil)

// PlayerSettingsRepository は port.PlayerSettingsRepo の PostgreSQL 実装。
type PlayerSettingsRepository struct {
	pool *pgxpool.Pool
}

// NewPlayerSettingsRepository は PlayerSettingsRepository を生成する。
func NewPlayerSettingsRepository(pool *pgxpool.Pool) *PlayerSettingsRepository {
	return &PlayerSettingsRepository{pool: pool}
}

// Insert は新規プレイヤー設定行を挿入する。
func (r *PlayerSettingsRepository) Insert(ctx context.Context, s *domain.PlayerSettings) error {
	now := time.Now()
	_, err := connFrom(ctx, r.pool).Exec(ctx,
		`INSERT INTO account.player_settings (player_id, language, bgm_volume, se_volume, push_enabled, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		s.PlayerID, s.Language, s.BgmVolume, s.SeVolume, s.PushEnabled, now,
	)
	if err != nil {
		return fmt.Errorf("insert player settings: %w", err)
	}
	s.UpdatedAt = now
	return nil
}

// Get はプレイヤーの設定を返す。該当なしは port.ErrNotFound。
func (r *PlayerSettingsRepository) Get(ctx context.Context, playerID string) (*domain.PlayerSettings, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, language, bgm_volume, se_volume, push_enabled, updated_at
		 FROM account.player_settings WHERE player_id = $1`,
		playerID,
	)

	var s domain.PlayerSettings
	err := row.Scan(&s.PlayerID, &s.Language, &s.BgmVolume, &s.SeVolume, &s.PushEnabled, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, port.ErrNotFound
		}
		return nil, fmt.Errorf("get player settings: %w", err)
	}
	return &s, nil
}

// UpdatePartial は patch の非 nil フィールドのみを更新する (COALESCE で現状維持)。
// updated_at は schema 側の BEFORE UPDATE トリガーが自動で書き換える。
// 全 nil patch は no-op だが trigger で updated_at だけ進むため、handler 層で全 nil を弾く前提。
func (r *PlayerSettingsRepository) UpdatePartial(ctx context.Context, playerID string, patch *port.PlayerSettingsPatch) error {
	ct, err := connFrom(ctx, r.pool).Exec(ctx,
		`UPDATE account.player_settings
		    SET language     = COALESCE($1, language),
		        bgm_volume   = COALESCE($2, bgm_volume),
		        se_volume    = COALESCE($3, se_volume),
		        push_enabled = COALESCE($4, push_enabled)
		  WHERE player_id = $5`,
		patch.Language, patch.BgmVolume, patch.SeVolume, patch.PushEnabled, playerID,
	)
	if err != nil {
		return fmt.Errorf("update partial player settings: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("player settings for player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}
