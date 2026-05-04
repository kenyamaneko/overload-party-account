package usecase

import (
	"context"
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/presenter"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// PlayerSettingsInteractor はプレイヤー設定の参照・更新を提供する。
type PlayerSettingsInteractor struct {
	repo port.PlayerSettingsRepo
}

// NewPlayerSettingsInteractor は PlayerSettingsInteractor を生成する。
func NewPlayerSettingsInteractor(repo port.PlayerSettingsRepo) *PlayerSettingsInteractor {
	return &PlayerSettingsInteractor{repo: repo}
}

// Get はプレイヤーの設定を返す。
// 行が無いのは Register 未実施または不整合なので、デフォルト値で隠さず port.ErrNotFound を伝播する。
func (s *PlayerSettingsInteractor) Get(ctx context.Context, playerID string) (*apiaccount.PlayerSettingsResponse, error) {
	settings, err := s.repo.Get(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("get player settings: %w", err)
	}
	return presenter.BuildPlayerSettingsResponse(settings), nil
}

// Update はプレイヤー設定を部分更新する。patch の非 nil フィールドだけが書き換わる。
func (s *PlayerSettingsInteractor) Update(ctx context.Context, playerID string, patch *port.PlayerSettingsPatch) error {
	return s.repo.UpdatePartial(ctx, playerID, patch)
}
