package service

import (
	"context"
	"strings"
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
	playerRepo, factionRepo, _, _ := newRealRepos()
	return NewPlayerService(playerRepo, newFakeGameConfigRepo(defaultValues), factionRepo)
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
			name:             "FreePlayer_UnderLimit",
			dailyBattleCount: 3,
			lastResetDate:    today(),
			wantCount:        3,
			wantLimit:        10,
			wantCanBattle:    true,
		},
		{
			name:             "FreePlayer_AtLimit",
			dailyBattleCount: 10,
			lastResetDate:    today(),
			wantCount:        10,
			wantLimit:        10,
			wantCanBattle:    false,
		},
		{
			name:             "PremiumPlayer",
			isPremium:        true,
			dailyBattleCount: 5,
			lastResetDate:    today(),
			wantCount:        0,
			wantLimit:        -1,
			wantCanBattle:    true,
		},
		{
			name:             "DateReset",
			dailyBattleCount: 7,
			lastResetDate:    yesterday(),
			wantCount:        0,
			wantLimit:        10,
			wantCanBattle:    true,
		},
		{
			name:             "FreePlayer_OverLimit",
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
	assert.True(t, strings.Contains(err.Error(), "game config"),
		"err %v should mention game config key", err)
}

func TestPlayerService_IncrementBattleCount(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		isPremium    bool
		seedCount    int64
		seedDate     civil.Date
		wantAfterCnt int64
		wantLimit    int64
	}{
		{
			name:         "free プレイヤーは 5→6",
			seedCount:    5,
			seedDate:     today(),
			wantAfterCnt: 6,
			wantLimit:    10,
		},
		{
			name:         "premium プレイヤーも DB にはカウントされるが制限は -1",
			isPremium:    true,
			seedCount:    29,
			seedDate:     today(),
			wantAfterCnt: 0, // premium は GetBattleLimit で DailyBattleCount=0 を返す仕様
			wantLimit:    -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice",
				tt.isPremium, 1, 0, tt.seedCount, tt.seedDate)

			svc := newPlayerTestService(nil)
			require.NoError(t, svc.IncrementBattleCount(ctx, testPlayerID1))

			resp, err := svc.GetBattleLimit(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAfterCnt, resp.DailyBattleCount)
			assert.Equal(t, tt.wantLimit, resp.DailyBattleLimit)
		})
	}
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
	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrNotFound)
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
	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrNotFound)
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
		{"below threshold", 0, 1, testExpWin, testExpWin, 1},
		{"exact level up", levelUpThreshold - testExpWin, 1, testExpWin, levelUpThreshold, 2},
		{"multiple level ups", 0, 1, int64(testExpCoeff * 4 * 4), int64(testExpCoeff * 4 * 4), 4},
		{"zero gain is noop", 100, 1, 0, 100, 1},
		{"negative gain is noop", 100, 1, -10, 100, 1},
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

	playerRepo, factionRepo, _, _ := newRealRepos()
	// coefficient を持たない fake を渡す。production では Firestore 読み取り失敗に相当。
	svc := NewPlayerService(playerRepo, newFakeGameConfigRepo(map[string]int64{}), factionRepo)

	err := svc.AwardExp(context.Background(), testPlayerID1, testExpWin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exp_formula_coefficient")
}

func TestPlayerService_AwardGameExp(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		winnerNum int64
		reason    string
		matchType string
		wantP1Exp int64
		wantP2Exp int64
		npcMatch  bool
	}{
		{"player1 wins", 1, "system_down", "pvp", testExpWin, testExpLoss, false},
		{"player2 wins", 2, "budget_zero", "pvp", testExpLoss, testExpWin, false},
		{"draw", 0, "draw", "pvp", testExpDraw, testExpDraw, false},
		{"npc match p1 wins", 1, "system_down", "npc", testExpWin, 0, true},
		{"npc match p1 loses", 2, "system_down", "npc", testExpLoss, 0, true},
		{"npc match draw", 0, "draw", "npc", testExpDraw, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice", false, 1, 0, 0, today())

			p2ID := testPlayerID2
			if tt.npcMatch {
				p2ID = "npc-easy"
			} else {
				seedPlayerWithState(t, testPlayerID2, "uid-2", "Bob", false, 1, 0, 0, today())
			}

			svc := newPlayerTestService(nil)
			require.NoError(t, svc.AwardGameExp(ctx, testPlayerID1, p2ID, tt.winnerNum, tt.reason, tt.matchType))

			got1, err := svc.GetPlayer(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantP1Exp, got1.Exp)

			if !tt.npcMatch {
				got2, err := svc.GetPlayer(ctx, testPlayerID2)
				require.NoError(t, err)
				assert.Equal(t, tt.wantP2Exp, got2.Exp)
			}
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
		{"below threshold", threshold(2) - 1, 1, 1},
		{"exact threshold", threshold(2), 1, 2},
		{"multiple level ups", threshold(4), 1, 4},
		{"stays at current", threshold(4) - 1, 3, 3},
		{"level 0 corrected to 1", 0, 0, 1},
		{"never decreases", 0, 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeLevel(tt.newExp, tt.currentLevel, testExpCoeff)
			assert.Equal(t, tt.wantLevel, got)
		})
	}
}

func TestPlayerService_NotFound(t *testing.T) {
	missing := "99999999-9999-9999-9999-999999999999"

	tests := []struct {
		name string
		fn   func(svc *PlayerService) error
	}{
		{
			name: "UpdateUsername_NotFound",
			fn: func(svc *PlayerService) error {
				_, err := svc.UpdateUsername(context.Background(), missing, "Bob")
				return err
			},
		},
		{
			name: "IncrementBattleCount_PlayerNotFound",
			fn: func(svc *PlayerService) error {
				return svc.IncrementBattleCount(context.Background(), missing)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			svc := newPlayerTestService(nil)
			err := tt.fn(svc)
			require.Error(t, err)
		})
	}
}
