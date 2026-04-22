//go:build integration

package service

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// ゲームバランス変更時にこの定数のみ修正すれば全アサーションに反映される。
const (
	testExpCoeff = 60
	testExpWin   = 40
	testExpLoss  = 20
	testExpDraw  = 30
)

func today() civil.Date {
	return civil.DateOf(time.Now().UTC().Add(4 * time.Hour))
}

func yesterday() civil.Date {
	d := today()
	return civil.Date{Year: d.Year, Month: d.Month, Day: d.Day - 1}
}

// newPlayerTestService は GameConfig fake + 実 repository で PlayerService を組む。
// defaultConfigValues をベースに overrides で上書きできる。
func newPlayerTestService(overrides map[string]int64) *PlayerService {
	defaultValues := map[string]int64{
		configKeyFreeDailyBattleLimit:  10,
		ConfigKeyExpFormulaCoefficient: testExpCoeff,
		"exp_win":                      testExpWin,
		"exp_loss":                     testExpLoss,
		"exp_draw":                     testExpDraw,
	}
	for k, v := range overrides {
		defaultValues[k] = v
	}
	playerRepo, factionRepo, _, tx := newRealRepos()
	return NewPlayerService(playerRepo, newFakeGameConfigRepo(defaultValues), factionRepo, tx)
}

func TestPlayerService_GetBattleLimit(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		isPremium        bool
		dailyBattleCount int64
		lastResetDate    civil.Date
		wantCount        int64
		wantLimit        int64
		wantCanBattle    bool
	}{
		{
			name:             "free プレイヤー: 上限未満なら対戦可能",
			isPremium:        false,
			dailyBattleCount: 3,
			lastResetDate:    today(),
			wantCount:        3,
			wantLimit:        10,
			wantCanBattle:    true,
		},
		{
			name:             "free プレイヤー: 上限到達で対戦不可",
			isPremium:        false,
			dailyBattleCount: 10,
			lastResetDate:    today(),
			wantCount:        10,
			wantLimit:        10,
			wantCanBattle:    false,
		},
		{
			name:             "premium プレイヤーは上限なしで常に対戦可能",
			isPremium:        true,
			dailyBattleCount: 5,
			lastResetDate:    today(),
			wantCount:        0,
			wantLimit:        -1,
			wantCanBattle:    true,
		},
		{
			name:             "日付が変われば free プレイヤーのカウントがリセットされる",
			isPremium:        false,
			dailyBattleCount: 7,
			lastResetDate:    yesterday(),
			wantCount:        0,
			wantLimit:        10,
			wantCanBattle:    true,
		},
		{
			name:             "free プレイヤー: 上限超過でも対戦不可",
			isPremium:        false,
			dailyBattleCount: 11,
			lastResetDate:    today(),
			wantCount:        11,
			wantLimit:        10,
			wantCanBattle:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice",
				tt.isPremium, 1, 0, tt.dailyBattleCount, tt.lastResetDate)

			svc := newPlayerTestService(nil)
			resp, err := svc.GetBattleLimit(ctx, testPlayerID1)
			require.NoError(t, err)

			assert.Equal(t, tt.wantCount, resp.DailyBattleCount)
			assert.Equal(t, tt.wantLimit, resp.DailyBattleLimit)
			assert.Equal(t, tt.wantCanBattle, resp.CanBattle)
		})
	}
}

func TestPlayerService_GetBattleLimit_FreeLimitZero_ReturnsError(t *testing.T) {
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newPlayerTestService(map[string]int64{configKeyFreeDailyBattleLimit: 0})

	_, err := svc.GetBattleLimit(context.Background(), testPlayerID1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "game config")
}

// IncrementBattleCount の正常系仕様:
//   - 日付が変わっていれば 1 にリセット
//   - free プレイヤーはインクリメント後のカウントが上限内なら通る
//   - premium プレイヤーは上限判定をスキップ（カウントは記録する）
func TestPlayerService_IncrementBattleCount(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		isPremium  bool
		seedCount  int64
		seedDate   civil.Date
		wantStored int64
	}{
		{
			name:       "free: 上限未満なら 1 加算される",
			isPremium:  false,
			seedCount:  5,
			seedDate:   today(),
			wantStored: 6,
		},
		{
			name:       "free: ちょうど上限に達するインクリメントは通る",
			isPremium:  false,
			seedCount:  9,
			seedDate:   today(),
			wantStored: 10,
		},
		{
			name:       "free: 日付が変わっていればリセットして 1 になる",
			isPremium:  false,
			seedCount:  9,
			seedDate:   yesterday(),
			wantStored: 1,
		},
		{
			name:       "premium: 上限を超えていても加算できる",
			isPremium:  true,
			seedCount:  29,
			seedDate:   today(),
			wantStored: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice",
				tt.isPremium, 1, 0, tt.seedCount, tt.seedDate)

			svc := newPlayerTestService(nil)
			require.NoError(t, svc.IncrementBattleCount(ctx, testPlayerID1))

			playerRepo, _, _, _ := newRealRepos()
			got, err := playerRepo.GetDailyBattle(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantStored, got.DailyBattleCount)
		})
	}
}

// free プレイヤーが上限到達後にインクリメントを試みると拒否され、カウントは据え置き。
func TestPlayerService_IncrementBattleCount_OverLimit_ReturnsError(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice",
		false, 1, 0, 10, today())

	svc := newPlayerTestService(nil)
	err := svc.IncrementBattleCount(ctx, testPlayerID1)
	require.ErrorIs(t, err, ErrBattleLimitExceeded)

	playerRepo, _, _, _ := newRealRepos()
	got, err := playerRepo.GetDailyBattle(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int64(10), got.DailyBattleCount)
}

func TestPlayerService_IncrementBattleCount_NotFound(t *testing.T) {
	sharedPg.Truncate(t)
	svc := newPlayerTestService(nil)

	err := svc.IncrementBattleCount(context.Background(), "99999999-9999-9999-9999-999999999999")
	require.ErrorIs(t, err, port.ErrNotFound)
}

func TestPlayerService_GetPlayer_Success(t *testing.T) {
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newPlayerTestService(nil)
	got, err := svc.GetPlayer(context.Background(), testPlayerID1)
	require.NoError(t, err)
	assert.Equal(t, testPlayerID1, got.PlayerID)
	assert.Equal(t, "Alice", got.Username)
}

func TestPlayerService_GetPlayer_NotFound_ReturnsError(t *testing.T) {
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newPlayerTestService(nil)
	_, err := svc.GetPlayer(context.Background(), "99999999-9999-9999-9999-999999999999")
	require.ErrorIs(t, err, port.ErrNotFound)
}

func TestPlayerService_UpdateUsername(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newPlayerTestService(nil)

	updated, err := svc.UpdateUsername(ctx, testPlayerID1, "Bob")
	require.NoError(t, err)
	assert.Equal(t, "Bob", updated.Username)

	got, err := svc.GetPlayer(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.Equal(t, "Bob", got.Username)
}

func TestPlayerService_UpdateUsername_NotFound(t *testing.T) {
	sharedPg.Truncate(t)
	svc := newPlayerTestService(nil)

	_, err := svc.UpdateUsername(context.Background(), "99999999-9999-9999-9999-999999999999", "Bob")
	require.ErrorIs(t, err, port.ErrNotFound)
}

func TestPlayerService_AwardExp(t *testing.T) {
	ctx := context.Background()
	levelUpThreshold := int64(testExpCoeff * 2 * 2)

	tests := []struct {
		name      string
		initExp   int64
		initLevel int64
		gain      int64
		wantExp   int64
		wantLevel int64
	}{
		{
			name:      "閾値未満なら同じレベルのまま",
			initExp:   0,
			initLevel: 1,
			gain:      testExpWin,
			wantExp:   testExpWin,
			wantLevel: 1,
		},
		{
			name:      "閾値ちょうどでレベルアップする",
			initExp:   levelUpThreshold - testExpWin,
			initLevel: 1,
			gain:      testExpWin,
			wantExp:   levelUpThreshold,
			wantLevel: 2,
		},
		{
			name:      "一度の加算で複数レベル上がる",
			initExp:   0,
			initLevel: 1,
			gain:      int64(testExpCoeff * 4 * 4),
			wantExp:   int64(testExpCoeff * 4 * 4),
			wantLevel: 4,
		},
		{
			name:      "加算量が 0 なら何もしない",
			initExp:   100,
			initLevel: 1,
			gain:      0,
			wantExp:   100,
			wantLevel: 1,
		},
		{
			name:      "加算量が負なら何もしない",
			initExp:   100,
			initLevel: 1,
			gain:      -10,
			wantExp:   100,
			wantLevel: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice",
				false, tt.initLevel, tt.initExp, 0, today())

			svc := newPlayerTestService(nil)
			require.NoError(t, svc.AwardExp(ctx, testPlayerID1, tt.gain))

			got, err := svc.GetPlayer(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantExp, got.Exp)
			assert.Equal(t, tt.wantLevel, got.Level)
		})
	}
}

func TestPlayerService_AwardExp_MissingCoefficient_ReturnsError(t *testing.T) {
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	playerRepo, factionRepo, _, tx := newRealRepos()
	// coefficient を持たない fake を渡す。production では Firestore 読み取り失敗に相当。
	svc := NewPlayerService(playerRepo, newFakeGameConfigRepo(map[string]int64{}), factionRepo, tx)

	err := svc.AwardExp(context.Background(), testPlayerID1, testExpWin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exp_formula_coefficient")
}

func TestPlayerService_AwardGameExp_PvP(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		winnerNum int64
		reason    string
		wantP1Exp int64
		wantP2Exp int64
	}{
		{
			name:      "プレイヤー1 が勝利",
			winnerNum: 1,
			reason:    "system_down",
			wantP1Exp: testExpWin,
			wantP2Exp: testExpLoss,
		},
		{
			name:      "プレイヤー2 が勝利",
			winnerNum: 2,
			reason:    "budget_zero",
			wantP1Exp: testExpLoss,
			wantP2Exp: testExpWin,
		},
		{
			name:      "引き分け",
			winnerNum: 0,
			reason:    "draw",
			wantP1Exp: testExpDraw,
			wantP2Exp: testExpDraw,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice", false, 1, 0, 0, today())
			seedPlayerWithState(t, testPlayerID2, "uid-2", "Bob", false, 1, 0, 0, today())

			svc := newPlayerTestService(nil)
			require.NoError(t, svc.AwardGameExp(ctx, testPlayerID1, testPlayerID2, tt.winnerNum, tt.reason, "pvp"))

			got1, err := svc.GetPlayer(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantP1Exp, got1.Exp)

			got2, err := svc.GetPlayer(ctx, testPlayerID2)
			require.NoError(t, err)
			assert.Equal(t, tt.wantP2Exp, got2.Exp)
		})
	}
}

func TestPlayerService_AwardGameExp_NPC(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		winnerNum int64
		reason    string
		wantP1Exp int64
	}{
		{
			name:      "プレイヤーが勝利",
			winnerNum: 1,
			reason:    "system_down",
			wantP1Exp: testExpWin,
		},
		{
			name:      "プレイヤーが敗北",
			winnerNum: 2,
			reason:    "system_down",
			wantP1Exp: testExpLoss,
		},
		{
			name:      "引き分け",
			winnerNum: 0,
			reason:    "draw",
			wantP1Exp: testExpDraw,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice", false, 1, 0, 0, today())

			svc := newPlayerTestService(nil)
			require.NoError(t, svc.AwardGameExp(ctx, testPlayerID1, "npc-easy", tt.winnerNum, tt.reason, "npc"))

			got1, err := svc.GetPlayer(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantP1Exp, got1.Exp)
		})
	}
}

func TestComputeLevel(t *testing.T) {
	threshold := func(level int64) int64 {
		return testExpCoeff * level * level
	}

	tests := []struct {
		name         string
		newExp       int64
		currentLevel int64
		wantLevel    int64
	}{
		{
			name:         "閾値未満なら同じレベルのまま",
			newExp:       threshold(2) - 1,
			currentLevel: 1,
			wantLevel:    1,
		},
		{
			name:         "閾値ちょうどでレベルアップする",
			newExp:       threshold(2),
			currentLevel: 1,
			wantLevel:    2,
		},
		{
			name:         "一度に複数レベル上がる",
			newExp:       threshold(4),
			currentLevel: 1,
			wantLevel:    4,
		},
		{
			name:         "次の閾値未満なら現在レベル据え置き",
			newExp:       threshold(4) - 1,
			currentLevel: 3,
			wantLevel:    3,
		},
		{
			name:         "レベル 0 は 1 に補正される",
			newExp:       0,
			currentLevel: 0,
			wantLevel:    1,
		},
		{
			name:         "レベルは下がらない",
			newExp:       0,
			currentLevel: 5,
			wantLevel:    5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeLevel(tt.newExp, tt.currentLevel, testExpCoeff)
			assert.Equal(t, tt.wantLevel, got)
		})
	}
}
