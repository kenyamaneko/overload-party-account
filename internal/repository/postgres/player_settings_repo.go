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

var _ port.PlayerSettingsRepo = (*PlayerSettingsRepository)(nil)

// PlayerSettingsRepository はプレイヤー設定の永続化の PostgreSQL 実装である。
type PlayerSettingsRepository struct {
	pool *pgxpool.Pool
}

// NewPlayerSettingsRepository は PlayerSettingsRepository を生成する。
func NewPlayerSettingsRepository(pool *pgxpool.Pool) *PlayerSettingsRepository {
	return &PlayerSettingsRepository{pool: pool}
}

// InsertPlayerSettings はプレイヤー設定を登録する。
func (r *PlayerSettingsRepository) InsertPlayerSettings(ctx context.Context, settings *domain.PlayerSettings) error {
	_, err := resolveDBTX(ctx, r.pool).Exec(ctx,
		`INSERT INTO account.player_settings (player_id, language, bgm_volume, se_volume, push_enabled, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		settings.PlayerID, settings.Language, settings.BgmVolume, settings.SeVolume, settings.IsPushEnabled, settings.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert player settings: %w", err)
	}
	return nil
}

// GetPlayerSettings はプレイヤー設定を返す。登録されていなければ port.ErrNotFound を返す。
func (r *PlayerSettingsRepository) GetPlayerSettings(ctx context.Context, playerID string) (*domain.PlayerSettings, error) {
	row := resolveDBTX(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, language, bgm_volume, se_volume, push_enabled, updated_at
		 FROM account.player_settings WHERE player_id = $1`,
		playerID,
	)

	var settings domain.PlayerSettings
	err := row.Scan(&settings.PlayerID, &settings.Language, &settings.BgmVolume, &settings.SeVolume, &settings.IsPushEnabled, &settings.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, port.ErrNotFound
		}
		return nil, fmt.Errorf("get player settings: %w", err)
	}
	return &settings, nil
}

// UpdatePlayerSettings は更新内容で指定された項目だけを更新する。登録されていなければ port.ErrNotFound を返す。
func (r *PlayerSettingsRepository) UpdatePlayerSettings(ctx context.Context, playerID string, patch *port.PlayerSettingsPatch) error {
	commandTag, err := resolveDBTX(ctx, r.pool).Exec(ctx,
		`UPDATE account.player_settings
		    SET language     = COALESCE($1, language),
		        bgm_volume   = COALESCE($2, bgm_volume),
		        se_volume    = COALESCE($3, se_volume),
		        push_enabled = COALESCE($4, push_enabled)
		  WHERE player_id = $5`,
		patch.Language, patch.BgmVolume, patch.SeVolume, patch.IsPushEnabled, playerID,
	)
	if err != nil {
		return fmt.Errorf("update player settings: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("player settings for player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}
