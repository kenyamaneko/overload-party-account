package repository

import (
	"context"
	"sync"
	"time"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// MockUserSettingsRepository はテスト・ローカルモード用の UserSettingsRepo インメモリ実装です。
type MockUserSettingsRepository struct {
	mu       sync.Mutex
	settings map[string]*apiaccount.UserSettings // playerID → UserSettings
}

var _ port.UserSettingsRepo = (*MockUserSettingsRepository)(nil)

// NewMockUserSettingsRepository は MockUserSettingsRepository を生成します。
func NewMockUserSettingsRepository() *MockUserSettingsRepository {
	return &MockUserSettingsRepository{
		settings: make(map[string]*apiaccount.UserSettings),
	}
}

// Get はプレイヤーのユーザー設定を返します。
func (r *MockUserSettingsRepository) Get(_ context.Context, playerID string) (*apiaccount.UserSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.settings[playerID]
	if !ok {
		return nil, nil
	}
	return s, nil
}

// Upsert はユーザー設定を挿入または更新します。
func (r *MockUserSettingsRepository) Upsert(_ context.Context, s *apiaccount.UserSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s.UpdatedAt = time.Now()
	r.settings[s.PlayerID] = s
	return nil
}
