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

// Insert は新規ユーザー設定行を挿入する。Register 時の初期化で呼び出される前提で、
// 全フィールドが非ゼロ値のアプリ層デフォルトで埋まっている想定。
func (r *UserSettingsRepository) Insert(ctx context.Context, s *apiaccount.UserSettings) error {
	now := time.Now()
	_, err := connFrom(ctx, r.pool).Exec(ctx,
		`INSERT INTO account.user_settings (player_id, language, bgm_volume, se_volume, push_enabled, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		s.PlayerID, s.Language, s.BgmVolume, s.SeVolume, s.PushEnabled, now,
	)
	if err != nil {
		return fmt.Errorf("insert user settings: %w", err)
	}
	s.UpdatedAt = now
	return nil
}

// UpdatePartial は patch の非 nil フィールドのみを更新する（COALESCE で現状維持）。
// 行が存在しなければ ErrNotFound を返す。呼び出し元は全 nil patch を事前に弾く想定。
//
// updated_at は schema 側の BEFORE UPDATE トリガー（trg_user_settings_updated_at）が自動で
// now() に書き換えるため、ここでは明示的にセットしない。全フィールドが nil の場合 UPDATE は
// 実質 no-op だがトリガーが発火して updated_at だけ進むので、handler 層で全 nil を弾く。
func (r *UserSettingsRepository) UpdatePartial(ctx context.Context, playerID string, patch *port.UserSettingsPatch) error {
	ct, err := connFrom(ctx, r.pool).Exec(ctx,
		`UPDATE account.user_settings
		    SET language     = COALESCE($1, language),
		        bgm_volume   = COALESCE($2, bgm_volume),
		        se_volume    = COALESCE($3, se_volume),
		        push_enabled = COALESCE($4, push_enabled)
		  WHERE player_id = $5`,
		patch.Language, patch.BgmVolume, patch.SeVolume, patch.PushEnabled, playerID,
	)
	if err != nil {
		return fmt.Errorf("update partial user settings: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("user settings for player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}
