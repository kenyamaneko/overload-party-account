package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.UserSettingsRepo = (*PgUserSettingsRepository)(nil)

// PgUserSettingsRepository は PostgreSQL を使用した UserSettingsRepo の実装です。
type PgUserSettingsRepository struct {
	pool *pgxpool.Pool
}

// NewPgUserSettingsRepository は PgUserSettingsRepository を生成します。
func NewPgUserSettingsRepository(pool *pgxpool.Pool) *PgUserSettingsRepository {
	return &PgUserSettingsRepository{pool: pool}
}

// Get はプレイヤーのユーザー設定を返します。該当なしは (nil, nil) を返します。
func (r *PgUserSettingsRepository) Get(ctx context.Context, playerID string) (*apiaccount.UserSettings, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, language, bgm_volume, se_volume, push_enabled, updated_at
		 FROM user_settings WHERE player_id = $1`,
		playerID,
	)

	var s apiaccount.UserSettings
	err := row.Scan(&s.PlayerID, &s.Language, &s.BgmVolume, &s.SeVolume, &s.PushEnabled, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user settings: %w", err)
	}
	return &s, nil
}

// Upsert はユーザー設定を挿入または更新します。
func (r *PgUserSettingsRepository) Upsert(ctx context.Context, s *apiaccount.UserSettings) error {
	db := connFrom(ctx, r.pool)

	now := time.Now()
	_, err := db.Exec(ctx,
		`INSERT INTO user_settings (player_id, language, bgm_volume, se_volume, push_enabled, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (player_id) DO UPDATE SET
		   language = EXCLUDED.language,
		   bgm_volume = EXCLUDED.bgm_volume,
		   se_volume = EXCLUDED.se_volume,
		   push_enabled = EXCLUDED.push_enabled,
		   updated_at = EXCLUDED.updated_at`,
		s.PlayerID, s.Language, s.BgmVolume, s.SeVolume, s.PushEnabled, now,
	)
	if err != nil {
		return fmt.Errorf("upsert user settings: %w", err)
	}
	s.UpdatedAt = now
	return nil
}
