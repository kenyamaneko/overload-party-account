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

var _ port.PlayerSettingsRepo = (*PlayerSettingsRepository)(nil)

// PlayerSettingsRepository は PostgreSQL を使用した PlayerSettingsRepo の実装である。
type PlayerSettingsRepository struct {
	pool *pgxpool.Pool
}

// NewPlayerSettingsRepository は PlayerSettingsRepository を生成する。
func NewPlayerSettingsRepository(pool *pgxpool.Pool) *PlayerSettingsRepository {
	return &PlayerSettingsRepository{pool: pool}
}

// Insert は新規プレイヤー設定行を挿入する。Register 時の初期化で呼び出される前提で、
// 全フィールドが非ゼロ値のアプリ層デフォルトで埋まっている想定。
func (r *PlayerSettingsRepository) Insert(ctx context.Context, s *apiaccount.PlayerSettings) error {
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

// Get はプレイヤーの設定を返す。該当なしは port.ErrNotFound でラップして返す。
func (r *PlayerSettingsRepository) Get(ctx context.Context, playerID string) (*apiaccount.PlayerSettings, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, language, bgm_volume, se_volume, push_enabled, updated_at
		 FROM account.player_settings WHERE player_id = $1`,
		playerID,
	)

	var s apiaccount.PlayerSettings
	err := row.Scan(&s.PlayerID, &s.Language, &s.BgmVolume, &s.SeVolume, &s.PushEnabled, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, port.ErrNotFound
		}
		return nil, fmt.Errorf("get player settings: %w", err)
	}
	return &s, nil
}

// UpdatePartial は patch の非 nil フィールドのみを更新する（COALESCE で現状維持）。
// 行が存在しなければ ErrNotFound を返す。呼び出し元は全 nil patch を事前に弾く想定。
//
// updated_at は schema 側の BEFORE UPDATE トリガー（trg_player_settings_updated_at）が自動で
// now() に書き換えるため、ここでは明示的にセットしない。全フィールドが nil の場合 UPDATE は
// 実質 no-op だがトリガーが発火して updated_at だけ進むので、handler 層で全 nil を弾く。
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
