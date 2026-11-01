package usecase

import (
	"context"
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// PlayerSettingsInteractor はプレイヤー設定の参照と更新を行う。
type PlayerSettingsInteractor struct {
	repo port.PlayerSettingsRepo
}

// NewPlayerSettingsInteractor は PlayerSettingsInteractor を生成する。
func NewPlayerSettingsInteractor(repo port.PlayerSettingsRepo) *PlayerSettingsInteractor {
	return &PlayerSettingsInteractor{repo: repo}
}

// GetPlayerSettings はプレイヤー設定を返す。登録されていなければ port.ErrNotFound を包んだエラーを返す。
func (si *PlayerSettingsInteractor) GetPlayerSettings(ctx context.Context, playerID string) (*domain.PlayerSettings, error) {
	settings, err := si.repo.GetPlayerSettings(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("get player settings: %w", err)
	}
	return settings, nil
}

// UpdatePlayerSettings はプレイヤー設定のうち、更新内容で指定された項目だけを更新する。
func (si *PlayerSettingsInteractor) UpdatePlayerSettings(ctx context.Context, playerID string, patch *port.PlayerSettingsPatch) error {
	return si.repo.UpdatePlayerSettings(ctx, playerID, patch)
}
