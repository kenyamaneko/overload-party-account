package repository

import (
	"context"
	"sync"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.FactionRepo = (*MockFactionRepository)(nil)

// MockFactionRepository はテスト用の FactionRepo インメモリ実装です。
type MockFactionRepository struct {
	mu       sync.Mutex
	factions map[string]map[string]string // playerID -> faction -> source
}

// NewMockFactionRepository は MockFactionRepository を生成します。
func NewMockFactionRepository() *MockFactionRepository {
	return &MockFactionRepository{
		factions: make(map[string]map[string]string),
	}
}

// AddPlayerFaction はプレイヤーファクションを追加します。
func (r *MockFactionRepository) AddPlayerFaction(_ context.Context, playerID, faction, source string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.factions[playerID] == nil {
		r.factions[playerID] = make(map[string]string)
	}
	if _, exists := r.factions[playerID][faction]; !exists {
		r.factions[playerID][faction] = source
	}
	return nil
}

// InsertInitial は PG の ON CONFLICT DO NOTHING セマンティクスを模倣します。
// 既存なら created=false、新規挿入なら created=true を返します。
func (r *MockFactionRepository) InsertInitial(_ context.Context, playerID, faction, source string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.factions[playerID] == nil {
		r.factions[playerID] = make(map[string]string)
	}
	if _, exists := r.factions[playerID][faction]; exists {
		return false, nil
	}
	r.factions[playerID][faction] = source
	return true, nil
}

// GetPlayerFactions はプレイヤーの所持ファクション一覧を返します。
func (r *MockFactionRepository) GetPlayerFactions(_ context.Context, playerID string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []string
	for f := range r.factions[playerID] {
		result = append(result, f)
	}
	return result, nil
}
