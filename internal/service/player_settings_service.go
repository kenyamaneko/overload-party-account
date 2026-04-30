package service

import (
	"context"
	"fmt"

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

// Get はプレイヤーの設定を返します。
// player_settings は Register と同一トランザクションで必ず INSERT される契約のため、
// 行が無いのは Register 未実施または不整合。デフォルト値で隠さず repo の port.ErrNotFound を
// そのまま伝播させます。
func (s *PlayerSettingsService) Get(ctx context.Context, playerID string) (*apiaccount.PlayerSettings, error) {
	settings, err := s.repo.Get(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("get player settings: %w", err)
	}
	return settings, nil
}

// Update はプレイヤー設定を部分更新します。patch の非 nil フィールドだけが書き換わります。
// 呼び出し元（handler）は空 patch を事前に弾く契約です。
func (s *PlayerSettingsService) Update(ctx context.Context, playerID string, patch *port.PlayerSettingsPatch) error {
	return s.repo.UpdatePartial(ctx, playerID, patch)
}
