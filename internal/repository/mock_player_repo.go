package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/civil"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// MockPlayerRepository はテスト・ローカルモード用の PlayerRepo インメモリ実装です。
type MockPlayerRepository struct {
	mu           sync.Mutex
	players      map[string]*apiaccount.Player // playerID → Player
	dailyBattles map[string]*apiaccount.PlayerDailyBattle
	byUID map[string]string // firebaseUID → playerID
}

var _ port.PlayerRepo = (*MockPlayerRepository)(nil)

// NewMockPlayerRepository は MockPlayerRepository を生成します。
func NewMockPlayerRepository() *MockPlayerRepository {
	return &MockPlayerRepository{
		players:      make(map[string]*apiaccount.Player),
		dailyBattles: make(map[string]*apiaccount.PlayerDailyBattle),
		byUID:        make(map[string]string),
	}
}

// Create はプレイヤーと日次バトルデータを作成します。
func (r *MockPlayerRepository) Create(ctx context.Context, player *apiaccount.Player, dailyBattle *apiaccount.PlayerDailyBattle) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byUID[player.FirebaseUID]; ok {
		return fmt.Errorf("player with firebase_uid %s already exists", player.FirebaseUID)
	}

	r.players[player.PlayerID] = player
	r.dailyBattles[player.PlayerID] = dailyBattle
	r.byUID[player.FirebaseUID] = player.PlayerID
	return nil
}

// FindByID はプレイヤー ID で検索します。
func (r *MockPlayerRepository) FindByID(ctx context.Context, playerID string) (*apiaccount.Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.players[playerID]
	if !ok {
		return nil, fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
	}
	return p, nil
}

// FindByFirebaseUID は Firebase UID で検索します。該当なしは (nil, nil) を返します。
func (r *MockPlayerRepository) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	pid, ok := r.byUID[firebaseUID]
	if !ok {
		return nil, nil
	}
	return r.players[pid], nil
}

// GetDailyBattle はプレイヤーの日次バトルデータを返します。
func (r *MockPlayerRepository) GetDailyBattle(ctx context.Context, playerID string) (*apiaccount.PlayerDailyBattle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	db, ok := r.dailyBattles[playerID]
	if !ok {
		return nil, nil
	}
	return db, nil
}

// IncrementDailyBattle は日次バトル回数をインクリメントします。
func (r *MockPlayerRepository) IncrementDailyBattle(ctx context.Context, playerID string, today civil.Date) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	db, ok := r.dailyBattles[playerID]
	if !ok {
		return 0, fmt.Errorf("daily battle data for player %s not found", playerID)
	}

	if db.LastResetDate != today {
		db.DailyBattleCount = 1
		db.LastResetDate = today
	} else {
		db.DailyBattleCount++
	}
	return db.DailyBattleCount, nil
}

// UpdateUsername はプレイヤー名を更新します。
func (r *MockPlayerRepository) UpdateUsername(ctx context.Context, playerID string, username string) (*apiaccount.Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.players[playerID]
	if !ok {
		return nil, fmt.Errorf("player %s not found", playerID)
	}
	p.Username = username
	return p, nil
}

// UpdatePremium はプレミアムステータスを更新します。
func (r *MockPlayerRepository) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.players[playerID]
	if !ok {
		return fmt.Errorf("player %s not found", playerID)
	}
	p.IsPremium = isPremium
	p.PremiumExpiresAt = expiresAt
	return nil
}

// UpdateFaction は選択ファクションを更新します。
func (r *MockPlayerRepository) UpdateFaction(ctx context.Context, playerID, faction string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.players[playerID]
	if !ok {
		return fmt.Errorf("player %s not found", playerID)
	}
	p.SelectedFaction = &faction
	return nil
}

// AddExp は経験値を加算しレベルを再計算します。
func (r *MockPlayerRepository) AddExp(ctx context.Context, playerID string, expGain int64, computeLevel func(newExp, currentLevel int64) int64) (*apiaccount.Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.players[playerID]
	if !ok {
		return nil, fmt.Errorf("player %s not found", playerID)
	}
	p.Exp += expGain
	p.Level = computeLevel(p.Exp, p.Level)
	return p, nil
}
