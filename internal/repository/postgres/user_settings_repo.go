package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

var _ port.UserSettingsRepo = (*UserSettingsRepository)(nil)

// UserSettingsRepository は PostgreSQL を使用した UserSettingsRepo の実装である。
type UserSettingsRepository struct {
	pool *pgxpool.Pool
}

// NewUserSettingsRepository は UserSettingsRepository を生成する。
func NewUserSettingsRepository(pool *pgxpool.Pool) *UserSettingsRepository {
	return &UserSettingsRepository{pool: pool}
}

// Get はプレイヤーのユーザー設定を返す。該当なしは (nil, nil) を返す。
func (r *UserSettingsRepository) Get(ctx context.Context, playerID string) (*apiaccount.UserSettings, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, language, bgm_volume, se_volume, push_enabled, updated_at
		 FROM account.user_settings WHERE player_id = $1`,
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

// Upsert はユーザー設定を挿入または更新する。
func (r *UserSettingsRepository) Upsert(ctx context.Context, s *apiaccount.UserSettings) error {
	db := connFrom(ctx, r.pool)

	now := time.Now()
	_, err := db.Exec(ctx,
		`INSERT INTO account.user_settings (player_id, language, bgm_volume, se_volume, push_enabled, updated_at)
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
