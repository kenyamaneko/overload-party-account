package service

import (
	"context"
	"fmt"
	"time"

	"github.com/kenyamaneko/overload-party-account/internal/model"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// UserSettingsService はユーザー設定の参照・更新を提供します。
type UserSettingsService struct {
	repo port.UserSettingsRepo
}

// NewUserSettingsService は UserSettingsService を生成します。
func NewUserSettingsService(repo port.UserSettingsRepo) *UserSettingsService {
	return &UserSettingsService{repo: repo}
}

// Get はプレイヤーのユーザー設定を返します。未登録ならデフォルト値を返します。
func (s *UserSettingsService) Get(ctx context.Context, playerID string) (*apiaccount.UserSettings, error) {
	settings, err := s.repo.Get(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("get user settings: %w", err)
	}
	if settings == nil {
		settings = &apiaccount.UserSettings{
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

// Update はユーザー設定を部分更新します。patch の非 nil フィールドだけが書き換わります。
// 呼び出し元（handler）は空 patch を事前に弾く契約です。
func (s *UserSettingsService) Update(ctx context.Context, playerID string, patch *port.UserSettingsPatch) error {
	return s.repo.UpdatePartial(ctx, playerID, patch)
}
