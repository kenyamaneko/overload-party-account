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

// Update はユーザー設定を更新します。
func (s *UserSettingsService) Update(ctx context.Context, settings *apiaccount.UserSettings) error {
	return s.repo.Upsert(ctx, settings)
}
