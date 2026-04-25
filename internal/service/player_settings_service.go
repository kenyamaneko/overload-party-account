package service

import (
	"context"
	"fmt"
	"time"

	"github.com/kenyamaneko/overload-party-account/internal/model"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// PlayerSettingsService はプレイヤー設定の参照・更新を提供します。
type PlayerSettingsService struct {
	repo port.PlayerSettingsRepo
}

// NewPlayerSettingsService は PlayerSettingsService を生成します。
func NewPlayerSettingsService(repo port.PlayerSettingsRepo) *PlayerSettingsService {
	return &PlayerSettingsService{repo: repo}
}

// Get はプレイヤーの設定を返します。未登録ならデフォルト値を返します。
func (s *PlayerSettingsService) Get(ctx context.Context, playerID string) (*apiaccount.PlayerSettings, error) {
	settings, err := s.repo.Get(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("get player settings: %w", err)
	}
	if settings == nil {
		settings = &apiaccount.PlayerSettings{
			PlayerID:    playerID,
			Language:    model.DefaultLanguage,
			BgmVolume:   model.DefaultBgmVolume,
			SeVolume:    model.DefaultSeVolume,
			PushEnabled: model.DefaultPushEnabled,
			UpdatedAt:   time.Now(),
		}
	}
	return settings, nil
}

// Update はプレイヤー設定を部分更新します。patch の非 nil フィールドだけが書き換わります。
// 呼び出し元（handler）は空 patch を事前に弾く契約です。
func (s *PlayerSettingsService) Update(ctx context.Context, playerID string, patch *port.PlayerSettingsPatch) error {
	return s.repo.UpdatePartial(ctx, playerID, patch)
}
