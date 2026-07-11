//go:build integration

package usecase

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

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
	return NewPlayerInteractor(playerRepo, playerRepo, playerRepo, playerRepo, playerViewRepo, newFakeGameConfigRepo(defaultValues), tx)
}

func TestGetBattleLimit(t *testing.T) {
	ctx := context.Background()

	t.Run("対戦上限の取得", func(t *testing.T) {
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
				name:          "free で当日 3 (上限 10 未満) のとき、対戦可能で count=3 になる",
				isPremium:     false,
				seedCount:     3,
				seedDate:      today(),
				wantCount:     3,
				wantLimit:     10,
				wantCanBattle: true,
			},
			{
				name:          "free で当日 10 (上限到達) のとき、対戦不可になる",
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
				name:          "premium で当日 5 のとき、上限なし (limit=-1) で対戦可能・count=5 を返す",
				isPremium:     true,
				seedCount:     5,
				seedDate:      today(),
				wantCount:     5,
				wantLimit:     -1,
				wantCanBattle: true,
			},
			{
				name:          "free で当日行が無い (前日履歴のみ) のとき、count=0 で対戦可能になる",
				isPremium:     false,
				seedCount:     7,
				seedDate:      yesterday(), // 別ゲーム日の履歴は当日カウントに影響しない
				wantCount:     0,
				wantLimit:     10,
				wantCanBattle: true,
			},
			{
				name:          "free で履歴自体が無いとき、count=0 で対戦可能になる",
				isPremium:     false,
				seedCount:     -1,
				wantCount:     0,
				wantLimit:     10,
				wantCanBattle: true,
			},
			{
				name:          "free で当日 11 (上限超過) のとき、対戦不可になる",
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
	})
}

func TestIncrementBattleCount(t *testing.T) {
	ctx := context.Background()

	t.Run("対戦カウントのインクリメント", func(t *testing.T) {
		// 当日 (computeCurrentGameDay) の行を日単位 UPSERT で加算し、別ゲーム日は独立、
		// free は上限内なら通り、premium は上限判定をスキップする (カウントは記録)。
		storedCases := []struct {
			name       string
			isPremium  bool
			seedCount  int64 // <0 のとき seed なし
			seedDate   civil.Date
			wantStored int64
		}{
			{
				name:       "free で当日 5 (上限未満) のとき、6 に加算される",
				isPremium:  false,
				seedCount:  5,
				seedDate:   today(),
				wantStored: 6,
			},
			{
				name:       "free で当日 9 → ちょうど上限 10 のとき、加算が通る",
				isPremium:  false,
				seedCount:  9,
				seedDate:   today(),
				wantStored: 10,
			},
			{
				name:       "free で当日行が無い (前日履歴のみ) のとき、1 で発生する",
				isPremium:  false,
				seedCount:  9,
				seedDate:   yesterday(),
				wantStored: 1,
			},
			{
				name:       "free で履歴自体が無いとき、1 で発生する",
				isPremium:  false,
				seedCount:  -1,
				wantStored: 1,
			},
			{
				name:       "premium で当日 29 (上限超過) のとき、30 に加算できる",
				isPremium:  true,
				seedCount:  29,
				seedDate:   today(),
				wantStored: 30,
			},
		}

		for _, tc := range storedCases {
			t.Run(tc.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayerWithState(t, testPlayerID1, "uid-1", "Alice",
					tc.isPremium, 1, 0, tc.seedCount, tc.seedDate)

				svc := newPlayerTestInteractor(nil)
				require.NoError(t, svc.IncrementBattleCount(ctx, testPlayerID1))

				playerRepo, _, _, _, _ := newRealRepos()
				got, err := playerRepo.GetDailyBattle(ctx, testPlayerID1, today())
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tc.wantStored, got.DailyBattleCount)
			})
		}

		t.Run("free で当日 10 (上限到達) からインクリメントするとき、ErrBattleLimitExceeded になりカウント据え置き", func(t *testing.T) {
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
		})

		t.Run("存在しない playerID のとき、port.ErrNotFound になる", func(t *testing.T) {
			sharedPg.Truncate(t)
			svc := newPlayerTestInteractor(nil)

			err := svc.IncrementBattleCount(context.Background(), "99999999-9999-9999-9999-999999999999")
			require.ErrorIs(t, err, port.ErrNotFound)
		})
	})
}

func TestGetPlayer(t *testing.T) {
	t.Run("プレイヤー応答の取得", func(t *testing.T) {
		t.Run("プレイヤーが存在するとき、その PlayerResponse を返す", func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			svc := newPlayerTestInteractor(nil)
			got, err := svc.GetPlayerResponse(context.Background(), testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, testPlayerID1, got.PlayerID)
			require.NotNil(t, got.Name)
			assert.Equal(t, "Alice", *got.Name)
		})

		t.Run("存在しない playerID のとき、port.ErrNotFound になる", func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			svc := newPlayerTestInteractor(nil)
			_, err := svc.GetPlayerResponse(context.Background(), "99999999-9999-9999-9999-999999999999")
			require.ErrorIs(t, err, port.ErrNotFound)
		})
	})
}

func TestUpdateName(t *testing.T) {
	ctx := context.Background()

	t.Run("name の更新", func(t *testing.T) {
		t.Run("有効な name のとき、更新され再取得でも反映される", func(t *testing.T) {
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
		})

		t.Run("空文字など無効な name のとき、repo に到達せず ErrInvalidName になる", func(t *testing.T) {
			// 詳細な境界値は domain/name_test.go で網羅済み。ここでは UpdateName 経路で
			// バリデーションが効き repo に到達しないことだけを確かめる。
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
		})

		t.Run("存在しない playerID のとき、port.ErrNotFound になる", func(t *testing.T) {
			sharedPg.Truncate(t)
			svc := newPlayerTestInteractor(nil)

			_, err := svc.UpdateName(context.Background(), "99999999-9999-9999-9999-999999999999", "Bob")
			require.ErrorIs(t, err, port.ErrNotFound)
		})
	})
}

func TestAwardExp(t *testing.T) {
	ctx := context.Background()
	levelUpThreshold := int64(testExpCoeff * 2 * 2)

	t.Run("経験値の付与", func(t *testing.T) {
		// AwardExp 固有の責務 (Tx 永続化 + 加算量 0 以下の早期リターン) のみ確認する。
		// レベル算出ロジック自体は ComputeLevel の単体テストで網羅済み。
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
				name:      "加算でレベルアップするとき、exp/level が永続化される",
				initExp:   levelUpThreshold - testExpWin,
				initLevel: 1,
				gain:      testExpWin,
				wantExp:   levelUpThreshold,
				wantLevel: 2,
			},
			{
				name:      "加算量が 0 のとき、何もしない",
				initExp:   100,
				initLevel: 1,
				gain:      0,
				wantExp:   100,
				wantLevel: 1,
			},
			{
				name:      "加算量が -10 のとき、何もしない",
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
	})
}

func TestAwardGameExp(t *testing.T) {
	ctx := context.Background()

	t.Run("ゲーム結果に応じた経験値付与", func(t *testing.T) {
		t.Run("PvP", func(t *testing.T) {
			tests := []struct {
				name      string
				winnerNum int64
				reason    string
				wantP1Exp int64
				wantP2Exp int64
			}{
				{
					name:      "プレイヤー1 が勝利するとき、P1 に exp_win・P2 に exp_loss が付与される",
					winnerNum: 1,
					reason:    "system_down",
					wantP1Exp: testExpWin,
					wantP2Exp: testExpLoss,
				},
				{
					name:      "プレイヤー2 が勝利するとき、P1 に exp_loss・P2 に exp_win が付与される",
					winnerNum: 2,
					reason:    "budget_zero",
					wantP1Exp: testExpLoss,
					wantP2Exp: testExpWin,
				},
				{
					name:      "引き分けのとき、両者に exp_draw が付与される",
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
					require.NoError(t, svc.AwardGameExp(ctx, testPlayerID1, testPlayerID2, tt.winnerNum, tt.reason, gamedesign.MatchTypePvp))

					got1, err := svc.GetPlayerResponse(ctx, testPlayerID1)
					require.NoError(t, err)
					assert.Equal(t, tt.wantP1Exp, got1.Exp)

					got2, err := svc.GetPlayerResponse(ctx, testPlayerID2)
					require.NoError(t, err)
					assert.Equal(t, tt.wantP2Exp, got2.Exp)
				})
			}
		})

		t.Run("NPC 戦", func(t *testing.T) {
			tests := []struct {
				name      string
				winnerNum int64
				reason    string
				wantP1Exp int64
			}{
				{
					name:      "プレイヤーが勝利するとき、exp_win が付与される",
					winnerNum: 1,
					reason:    "system_down",
					wantP1Exp: testExpWin,
				},
				{
					name:      "プレイヤーが敗北するとき、exp_loss が付与される",
					winnerNum: 2,
					reason:    "system_down",
					wantP1Exp: testExpLoss,
				},
				{
					name:      "引き分けのとき、exp_draw が付与される",
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
					require.NoError(t, svc.AwardGameExp(ctx, testPlayerID1, "npc-easy", tt.winnerNum, tt.reason, gamedesign.MatchTypeNpc))

					got1, err := svc.GetPlayerResponse(ctx, testPlayerID1)
					require.NoError(t, err)
					assert.Equal(t, tt.wantP1Exp, got1.Exp)
				})
			}
		})
	})
}
