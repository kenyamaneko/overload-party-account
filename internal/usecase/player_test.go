//go:build integration

package usecase

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
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

// newPlayerTestInteractor は GameConfig fake + 実 repository で PlayerInteractor を組む。
// defaultConfigValues をベースに overrides で上書きできる。
func newPlayerTestInteractor(overrides map[string]int64) *PlayerInteractor {
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
	playerRepo, playerViewRepo, _, _, tx := newRealRepos()
	return NewPlayerInteractor(playerRepo, playerViewRepo, newFakeGameConfigRepo(defaultValues), tx)
}

func TestGetBattleLimit(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		isPremium     bool
		seedCount     int64 // <0 のときは player_daily_battle に行を作らない
		seedDate      civil.Date
		wantCount     int64
		wantLimit     int64
		wantCanBattle bool
	}{
		{
			name:          "free プレイヤー: 上限未満なら対戦可能",
			isPremium:     false,
			seedCount:     3,
			seedDate:      today(),
			wantCount:     3,
			wantLimit:     10,
			wantCanBattle: true,
		},
		{
			name:          "free プレイヤー: 上限到達で対戦不可",
			isPremium:     false,
			seedCount:     10,
			seedDate:      today(),
			wantCount:     10,
			wantLimit:     10,
			wantCanBattle: false,
		},
		{
			// premium でも実カウントを返す (limit=-1 / can_battle=true は変わらない)。
			// データ分析のため count を 0 で潰さない。
			name:          "premium プレイヤーは上限なしで常に対戦可能、ただしカウントは集計用に返す",
			isPremium:     true,
			seedCount:     5,
			seedDate:      today(),
			wantCount:     5,
			wantLimit:     -1,
			wantCanBattle: true,
		},
		{
			name:          "free プレイヤー: 当日の行が無ければカウント 0 (新ゲーム日)",
			isPremium:     false,
			seedCount:     7,
			seedDate:      yesterday(), // 別ゲーム日の履歴は当日カウントに影響しない
			wantCount:     0,
			wantLimit:     10,
			wantCanBattle: true,
		},
		{
			name:          "free プレイヤー: 履歴自体が無くてもカウント 0 で対戦可能",
			isPremium:     false,
			seedCount:     -1,
			wantCount:     0,
			wantLimit:     10,
			wantCanBattle: true,
		},
		{
			name:          "free プレイヤー: 上限超過でも対戦不可",
			isPremium:     false,
			seedCount:     11,
			seedDate:      today(),
			wantCount:     11,
			wantLimit:     10,
			wantCanBattle: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice",
				tt.isPremium, 1, 0, tt.seedCount, tt.seedDate)

			svc := newPlayerTestInteractor(nil)
			resp, err := svc.GetBattleLimit(ctx, testPlayerID1)
			require.NoError(t, err)

			assert.Equal(t, tt.wantCount, resp.DailyBattleCount)
			assert.Equal(t, tt.wantLimit, resp.DailyBattleLimit)
			assert.Equal(t, tt.wantCanBattle, resp.CanBattle)
		})
	}
}

// IncrementBattleCount の正常系仕様:
//   - 当日 (currentGameDay) の行が無ければ count=1 で発生
//   - 当日の行があれば +1 加算
//   - 別ゲーム日の履歴は当日カウントに影響しない (UPSERT が日単位で独立)
//   - free プレイヤーはインクリメント後のカウントが上限内なら通る
//   - premium プレイヤーは上限判定をスキップ（カウントは記録する）
func TestIncrementBattleCount(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		isPremium  bool
		seedCount  int64 // <0 のとき seed なし
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
			name:       "free: 当日の行が無ければ 1 で発生する (前日履歴は無関係)",
			isPremium:  false,
			seedCount:  9,
			seedDate:   yesterday(),
			wantStored: 1,
		},
		{
			name:       "free: 履歴自体が無ければ 1 で発生する",
			isPremium:  false,
			seedCount:  -1,
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

			svc := newPlayerTestInteractor(nil)
			require.NoError(t, svc.IncrementBattleCount(ctx, testPlayerID1))

			playerRepo, _, _, _, _ := newRealRepos()
			got, err := playerRepo.GetDailyBattle(ctx, testPlayerID1, today())
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantStored, got.DailyBattleCount)
		})
	}
}

// free プレイヤーが上限到達後にインクリメントを試みると拒否され、カウントは据え置き。
func TestIncrementBattleCount_OverLimit_ReturnsError(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice",
		false, 1, 0, 10, today())

	svc := newPlayerTestInteractor(nil)
	err := svc.IncrementBattleCount(ctx, testPlayerID1)
	require.ErrorIs(t, err, ErrBattleLimitExceeded)

	playerRepo, _, _, _, _ := newRealRepos()
	got, err := playerRepo.GetDailyBattle(ctx, testPlayerID1, today())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int64(10), got.DailyBattleCount)
}

func TestIncrementBattleCount_NotFound(t *testing.T) {
	sharedPg.Truncate(t)
	svc := newPlayerTestInteractor(nil)

	err := svc.IncrementBattleCount(context.Background(), "99999999-9999-9999-9999-999999999999")
	require.ErrorIs(t, err, port.ErrNotFound)
}

func TestGetPlayer_Success(t *testing.T) {
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newPlayerTestInteractor(nil)
	got, err := svc.GetPlayerResponse(context.Background(), testPlayerID1)
	require.NoError(t, err)
	assert.Equal(t, testPlayerID1, got.PlayerID)
	require.NotNil(t, got.Name)
	assert.Equal(t, "Alice", *got.Name)
}

func TestGetPlayer_NotFound_ReturnsError(t *testing.T) {
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newPlayerTestInteractor(nil)
	_, err := svc.GetPlayerResponse(context.Background(), "99999999-9999-9999-9999-999999999999")
	require.ErrorIs(t, err, port.ErrNotFound)
}

func TestUpdateName(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newPlayerTestInteractor(nil)

	updated, err := svc.UpdateName(ctx, testPlayerID1, "Bob")
	require.NoError(t, err)
	require.NotNil(t, updated.Name)
	assert.Equal(t, "Bob", *updated.Name)

	got, err := svc.GetPlayerResponse(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got.Name)
	assert.Equal(t, "Bob", *got.Name)
}

// 業務バリデーション違反は repo に到達せず ErrInvalidName を返す契約を固定する。
// 詳細な境界値は model/name_test.go で網羅しているため、ここでは UpdateName 経路で
// バリデーションが効いていることだけを 1 ケースで確認する。
func TestUpdateName_InvalidName_ReturnsErrInvalidName(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newPlayerTestInteractor(nil)
	_, err := svc.UpdateName(ctx, testPlayerID1, "")
	require.ErrorIs(t, err, domain.ErrInvalidName)

	// repo に到達していないこと: name はシード値のまま。
	got, err := svc.GetPlayerResponse(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got.Name)
	assert.Equal(t, "Alice", *got.Name)
}

func TestUpdateName_NotFound(t *testing.T) {
	sharedPg.Truncate(t)
	svc := newPlayerTestInteractor(nil)

	_, err := svc.UpdateName(context.Background(), "99999999-9999-9999-9999-999999999999", "Bob")
	require.ErrorIs(t, err, port.ErrNotFound)
}

// AwardExp 固有の責務 (Tx 永続化 + 早期リターン) のみ確認する。
// レベル算出ロジック自体は ComputeLevel の単体テストで網羅済み。
func TestAwardExp(t *testing.T) {
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
			// レベル算出結果が DB に永続化されることを 1 ケースで担保する。
			name:      "加算後の exp/level が永続化される",
			initExp:   levelUpThreshold - testExpWin,
			initLevel: 1,
			gain:      testExpWin,
			wantExp:   levelUpThreshold,
			wantLevel: 2,
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

			svc := newPlayerTestInteractor(nil)
			require.NoError(t, svc.AwardExp(ctx, testPlayerID1, tt.gain))

			got, err := svc.GetPlayerResponse(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantExp, got.Exp)
			assert.Equal(t, tt.wantLevel, got.Level)
		})
	}
}

func TestAwardGameExp_PvP(t *testing.T) {
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

			svc := newPlayerTestInteractor(nil)
			require.NoError(t, svc.AwardGameExp(ctx, testPlayerID1, testPlayerID2, tt.winnerNum, tt.reason, "pvp"))

			got1, err := svc.GetPlayerResponse(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantP1Exp, got1.Exp)

			got2, err := svc.GetPlayerResponse(ctx, testPlayerID2)
			require.NoError(t, err)
			assert.Equal(t, tt.wantP2Exp, got2.Exp)
		})
	}
}

func TestAwardGameExp_NPC(t *testing.T) {
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

			svc := newPlayerTestInteractor(nil)
			require.NoError(t, svc.AwardGameExp(ctx, testPlayerID1, "npc-easy", tt.winnerNum, tt.reason, "npc"))

			got1, err := svc.GetPlayerResponse(ctx, testPlayerID1)
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
