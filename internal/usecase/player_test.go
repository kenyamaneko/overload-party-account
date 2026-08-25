//go:build integration

package usecase_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamelogic "github.com/kenyamaneko/overload-party-battle/packages/game-logic-constants-go"
	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

func TestPlayerInteractor_UpdatePremium(t *testing.T) {
	t.Run("PlayerInteractor", func(t *testing.T) {
		t.Run("UpdatePremium", func(t *testing.T) {
			t.Run("expiresAtMillisが指定されているとき、UNIXミリ秒を絶対時刻に変換してプレミアム有効期限として保存する", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-premium-1")
				expiresAt := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
				millis := expiresAt.UnixMilli()

				err := interactor.UpdatePremium(context.Background(), playerID, true, &millis)

				require.NoError(t, err)
				resp, err := interactor.GetPlayerResponse(context.Background(), playerID)
				require.NoError(t, err)
				require.NotNil(t, resp.PremiumExpiresAt)
				assert.WithinDuration(t, expiresAt, *resp.PremiumExpiresAt, time.Millisecond)
			})

			t.Run("expiresAtMillisが指定されていない(nil)とき、プレミアム有効期限は未設定(nil)として保存する", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-premium-2")

				err := interactor.UpdatePremium(context.Background(), playerID, true, nil)

				require.NoError(t, err)
				resp, err := interactor.GetPlayerResponse(context.Background(), playerID)
				require.NoError(t, err)
				assert.Nil(t, resp.PremiumExpiresAt)
			})

			t.Run("対象プレイヤーが存在しないとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())

				err := interactor.UpdatePremium(context.Background(), uuid.NewString(), true, nil)

				assert.ErrorIs(t, err, usecase.ErrNotFound)
			})
		})
	})
}

func TestPlayerInteractor_UpdateName(t *testing.T) {
	t.Run("PlayerInteractor", func(t *testing.T) {
		t.Run("UpdateName", func(t *testing.T) {
			t.Run("表示名が表示名バリデーションの規定に違反するとき、更新せずエラーを返す", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-name-1")

				_, err := interactor.UpdateName(context.Background(), playerID, "")

				require.Error(t, err)
			})

			t.Run("表示名が有効なとき、表示名を更新し、更新後のプレイヤー情報を返す", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-name-2")

				resp, err := interactor.UpdateName(context.Background(), playerID, "新しい名前")

				require.NoError(t, err)
				require.NotNil(t, resp.Name)
				assert.Equal(t, "新しい名前", *resp.Name)
			})
		})
	})
}

func TestPlayerInteractor_ValidateNameForOnboarding(t *testing.T) {
	t.Run("PlayerInteractor", func(t *testing.T) {
		t.Run("ValidateNameForOnboarding", func(t *testing.T) {
			t.Run("対象プレイヤーが存在しないとき、エラーを返す", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())

				err := interactor.ValidateNameForOnboarding(context.Background(), uuid.NewString(), "プレイヤー")

				require.Error(t, err)
			})

			t.Run("プレイヤーが存在し、表示名が表示名バリデーションの規定に違反するとき、エラーを返し、対象プレイヤーの表示名は呼び出し前の値のまま変わらない", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-validate-1")
				_, err := interactor.UpdateName(context.Background(), playerID, "呼び出し前の名前")
				require.NoError(t, err)

				err = interactor.ValidateNameForOnboarding(context.Background(), playerID, "")

				require.Error(t, err)
				resp, err := interactor.GetPlayerResponse(context.Background(), playerID)
				require.NoError(t, err)
				require.NotNil(t, resp.Name)
				assert.Equal(t, "呼び出し前の名前", *resp.Name)
			})

			t.Run("プレイヤーが存在し、呼び出し前に保存されている表示名とは異なる有効な表示名を指定したとき、エラーを返さず、対象プレイヤーの表示名は呼び出し前の値のまま変わらない", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-validate-2")
				_, err := interactor.UpdateName(context.Background(), playerID, "呼び出し前の名前")
				require.NoError(t, err)

				err = interactor.ValidateNameForOnboarding(context.Background(), playerID, "別の有効な名前")

				require.NoError(t, err)
				resp, err := interactor.GetPlayerResponse(context.Background(), playerID)
				require.NoError(t, err)
				require.NotNil(t, resp.Name)
				assert.Equal(t, "呼び出し前の名前", *resp.Name)
			})
		})
	})
}

func TestPlayerInteractor_GetBattleLimit(t *testing.T) {
	t.Run("PlayerInteractor", func(t *testing.T) {
		t.Run("GetBattleLimit", func(t *testing.T) {
			t.Run("プレミアム会員のとき、1日の対戦回数の上限は無制限(-1)になる", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-limit-1")
				require.NoError(t, interactor.UpdatePremium(context.Background(), playerID, true, nil))

				resp, err := interactor.GetBattleLimit(context.Background(), playerID)

				require.NoError(t, err)
				assert.Equal(t, int64(-1), resp.DailyBattleLimit)
			})

			t.Run("プレミアム会員のとき、CanBattleは常にtrueになる", func(t *testing.T) {
				values := validGameConfigValues()
				values["free_daily_battle_limit"] = 1
				interactor := newTestPlayerInteractor(t, values)
				playerID := registerTestPlayer(t, "firebase-limit-2")
				require.NoError(t, interactor.UpdatePremium(context.Background(), playerID, true, nil))
				require.NoError(t, interactor.IncrementBattleCount(context.Background(), playerID))
				require.NoError(t, interactor.IncrementBattleCount(context.Background(), playerID))

				resp, err := interactor.GetBattleLimit(context.Background(), playerID)

				require.NoError(t, err)
				assert.True(t, resp.CanBattle)
			})

			t.Run("プレミアム会員であっても、当日の対戦回数はそのまま(実カウントを)返す", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-limit-3")
				require.NoError(t, interactor.UpdatePremium(context.Background(), playerID, true, nil))
				require.NoError(t, interactor.IncrementBattleCount(context.Background(), playerID))

				resp, err := interactor.GetBattleLimit(context.Background(), playerID)

				require.NoError(t, err)
				assert.Equal(t, int64(1), resp.DailyBattleCount)
			})

			t.Run("プレミアム会員でなく、当日の対戦回数が設定された上限未満のとき、CanBattleはtrueになる", func(t *testing.T) {
				values := validGameConfigValues()
				values["free_daily_battle_limit"] = 2
				interactor := newTestPlayerInteractor(t, values)
				playerID := registerTestPlayer(t, "firebase-limit-4")
				require.NoError(t, interactor.IncrementBattleCount(context.Background(), playerID))

				resp, err := interactor.GetBattleLimit(context.Background(), playerID)

				require.NoError(t, err)
				assert.True(t, resp.CanBattle)
			})

			t.Run("プレミアム会員でなく、当日の対戦回数が設定された上限に達しているとき、CanBattleはfalseになる", func(t *testing.T) {
				values := validGameConfigValues()
				values["free_daily_battle_limit"] = 2
				interactor := newTestPlayerInteractor(t, values)
				playerID := registerTestPlayer(t, "firebase-limit-5")
				require.NoError(t, interactor.IncrementBattleCount(context.Background(), playerID))
				require.NoError(t, interactor.IncrementBattleCount(context.Background(), playerID))

				resp, err := interactor.GetBattleLimit(context.Background(), playerID)

				require.NoError(t, err)
				assert.False(t, resp.CanBattle)
			})

			t.Run("当日まだ一度も対戦していない(対戦回数の記録が無い)とき、対戦回数は0として扱われる", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-limit-6")

				resp, err := interactor.GetBattleLimit(context.Background(), playerID)

				require.NoError(t, err)
				assert.Equal(t, int64(0), resp.DailyBattleCount)
			})

			t.Run("対象プレイヤーが存在しないとき、エラーを返す", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())

				_, err := interactor.GetBattleLimit(context.Background(), uuid.NewString())

				require.Error(t, err)
			})
		})
	})
}

func TestPlayerInteractor_IncrementBattleCount(t *testing.T) {
	t.Run("PlayerInteractor", func(t *testing.T) {
		t.Run("IncrementBattleCount", func(t *testing.T) {
			t.Run("プレミアム会員のとき、上限判定を行わずに当日の対戦回数を1加算する", func(t *testing.T) {
				values := validGameConfigValues()
				values["free_daily_battle_limit"] = 1
				interactor := newTestPlayerInteractor(t, values)
				playerID := registerTestPlayer(t, "firebase-increment-1")
				require.NoError(t, interactor.UpdatePremium(context.Background(), playerID, true, nil))
				require.NoError(t, interactor.IncrementBattleCount(context.Background(), playerID))

				err := interactor.IncrementBattleCount(context.Background(), playerID)

				require.NoError(t, err)
				resp, err := interactor.GetBattleLimit(context.Background(), playerID)
				require.NoError(t, err)
				assert.Equal(t, int64(2), resp.DailyBattleCount)
			})

			t.Run("プレミアム会員でなく、加算後も設定された上限以下のとき、当日の対戦回数を1加算する", func(t *testing.T) {
				values := validGameConfigValues()
				values["free_daily_battle_limit"] = 2
				interactor := newTestPlayerInteractor(t, values)
				playerID := registerTestPlayer(t, "firebase-increment-2")

				err := interactor.IncrementBattleCount(context.Background(), playerID)

				require.NoError(t, err)
				resp, err := interactor.GetBattleLimit(context.Background(), playerID)
				require.NoError(t, err)
				assert.Equal(t, int64(1), resp.DailyBattleCount)
			})

			t.Run("プレミアム会員でなく、加算により設定された上限を超えるとき、対戦回数を加算せずErrBattleLimitExceededを返す", func(t *testing.T) {
				values := validGameConfigValues()
				values["free_daily_battle_limit"] = 1
				interactor := newTestPlayerInteractor(t, values)
				playerID := registerTestPlayer(t, "firebase-increment-3")
				require.NoError(t, interactor.IncrementBattleCount(context.Background(), playerID))

				err := interactor.IncrementBattleCount(context.Background(), playerID)

				assert.ErrorIs(t, err, usecase.ErrBattleLimitExceeded)
				resp, err := interactor.GetBattleLimit(context.Background(), playerID)
				require.NoError(t, err)
				assert.Equal(t, int64(1), resp.DailyBattleCount)
			})

			t.Run("対象プレイヤーが存在しないとき、エラーを返す", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())

				err := interactor.IncrementBattleCount(context.Background(), uuid.NewString())

				require.Error(t, err)
			})
		})
	})
}

func TestPlayerInteractor_RevertBattleCount(t *testing.T) {
	t.Run("PlayerInteractor", func(t *testing.T) {
		t.Run("RevertBattleCount", func(t *testing.T) {
			t.Run("同一game_idに対する初回の呼び出しでは、消費時刻から算出したゲーム日の両プレイヤーの対戦回数をそれぞれ1減算する", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				player1ID := registerTestPlayer(t, "firebase-revert-1a")
				player2ID := registerTestPlayer(t, "firebase-revert-1b")
				require.NoError(t, interactor.IncrementBattleCount(context.Background(), player1ID))
				require.NoError(t, interactor.IncrementBattleCount(context.Background(), player2ID))

				err := interactor.RevertBattleCount(context.Background(), "game-1", time.Now().UnixMilli(), player1ID, player2ID)

				require.NoError(t, err)
				resp1, err := interactor.GetBattleLimit(context.Background(), player1ID)
				require.NoError(t, err)
				assert.Equal(t, int64(0), resp1.DailyBattleCount)
				resp2, err := interactor.GetBattleLimit(context.Background(), player2ID)
				require.NoError(t, err)
				assert.Equal(t, int64(0), resp2.DailyBattleCount)
			})

			t.Run("同一game_idに対する2回目以降の呼び出しでは、対戦回数の減算を行わず、成功として扱う(冪等)", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				player1ID := registerTestPlayer(t, "firebase-revert-2a")
				player2ID := registerTestPlayer(t, "firebase-revert-2b")
				for i := 0; i < 3; i++ {
					require.NoError(t, interactor.IncrementBattleCount(context.Background(), player1ID))
					require.NoError(t, interactor.IncrementBattleCount(context.Background(), player2ID))
				}
				consumedAtMillis := time.Now().UnixMilli()
				require.NoError(t, interactor.RevertBattleCount(context.Background(), "game-2", consumedAtMillis, player1ID, player2ID))

				err := interactor.RevertBattleCount(context.Background(), "game-2", consumedAtMillis, player1ID, player2ID)

				require.NoError(t, err)
				resp1, err := interactor.GetBattleLimit(context.Background(), player1ID)
				require.NoError(t, err)
				assert.Equal(t, int64(2), resp1.DailyBattleCount)
				resp2, err := interactor.GetBattleLimit(context.Background(), player2ID)
				require.NoError(t, err)
				assert.Equal(t, int64(2), resp2.DailyBattleCount)
			})
		})
	})
}

func TestPlayerInteractor_AwardExp(t *testing.T) {
	t.Run("PlayerInteractor", func(t *testing.T) {
		t.Run("AwardExp", func(t *testing.T) {
			t.Run("expGainが0以下のとき、経験値・レベルは変化しない", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-exp-1")

				err := interactor.AwardExp(context.Background(), playerID, 0)

				require.NoError(t, err)
				resp, err := interactor.GetPlayerResponse(context.Background(), playerID)
				require.NoError(t, err)
				assert.Equal(t, int64(0), resp.Exp)
				assert.Equal(t, int64(1), resp.Level)
			})

			t.Run("expGainが正のとき、累計経験値に加算し、加算後の累計経験値に基づいてレベルを再計算する", func(t *testing.T) {
				// coeff=100, level1のレベル2必要経験値は100*2^2=400
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-exp-2")

				err := interactor.AwardExp(context.Background(), playerID, 400)

				require.NoError(t, err)
				resp, err := interactor.GetPlayerResponse(context.Background(), playerID)
				require.NoError(t, err)
				assert.Equal(t, int64(400), resp.Exp)
				assert.Equal(t, int64(2), resp.Level)
			})

			t.Run("対象プレイヤーが存在しないとき、エラーを返す", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())

				err := interactor.AwardExp(context.Background(), uuid.NewString(), 10)

				require.Error(t, err)
			})

			t.Run("同一プレイヤーに対して2つの経験値付与が同時に実行されても、両方の加算が反映され、一方が失われない", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-exp-concurrent")

				var wg sync.WaitGroup
				errs := make([]error, 2)
				wg.Add(2)
				go func() {
					defer wg.Done()
					errs[0] = interactor.AwardExp(context.Background(), playerID, 10)
				}()
				go func() {
					defer wg.Done()
					errs[1] = interactor.AwardExp(context.Background(), playerID, 10)
				}()
				wg.Wait()

				require.NoError(t, errs[0])
				require.NoError(t, errs[1])
				resp, err := interactor.GetPlayerResponse(context.Background(), playerID)
				require.NoError(t, err)
				assert.Equal(t, int64(20), resp.Exp)
			})
		})
	})
}

func TestPlayerInteractor_AwardGameExp(t *testing.T) {
	t.Run("PlayerInteractor", func(t *testing.T) {
		t.Run("AwardGameExp", func(t *testing.T) {
			t.Run("reasonが引き分けのとき、winner_numの値に関わらず両プレイヤーに引き分け時経験値(exp_draw)を付与する", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				player1ID := registerTestPlayer(t, "firebase-game-exp-1a")
				player2ID := registerTestPlayer(t, "firebase-game-exp-1b")

				err := interactor.AwardGameExp(context.Background(), player1ID, player2ID, 1, gamelogic.WinReasonDraw, gamedesign.MatchTypePvp)

				require.NoError(t, err)
				resp1, err := interactor.GetPlayerResponse(context.Background(), player1ID)
				require.NoError(t, err)
				assert.Equal(t, int64(20), resp1.Exp)
				resp2, err := interactor.GetPlayerResponse(context.Background(), player2ID)
				require.NoError(t, err)
				assert.Equal(t, int64(20), resp2.Exp)
			})

			t.Run("winner_numが0のとき、reasonの値に関わらず両プレイヤーに引き分け時経験値(exp_draw)を付与する", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				player1ID := registerTestPlayer(t, "firebase-game-exp-2a")
				player2ID := registerTestPlayer(t, "firebase-game-exp-2b")

				err := interactor.AwardGameExp(context.Background(), player1ID, player2ID, 0, gamelogic.WinReasonBudgetZero, gamedesign.MatchTypePvp)

				require.NoError(t, err)
				resp1, err := interactor.GetPlayerResponse(context.Background(), player1ID)
				require.NoError(t, err)
				assert.Equal(t, int64(20), resp1.Exp)
				resp2, err := interactor.GetPlayerResponse(context.Background(), player2ID)
				require.NoError(t, err)
				assert.Equal(t, int64(20), resp2.Exp)
			})

			t.Run("winner_numが1のとき、プレイヤー1に勝利時経験値を、プレイヤー2に敗北時経験値を付与する", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				player1ID := registerTestPlayer(t, "firebase-game-exp-3a")
				player2ID := registerTestPlayer(t, "firebase-game-exp-3b")

				err := interactor.AwardGameExp(context.Background(), player1ID, player2ID, 1, gamelogic.WinReasonBudgetZero, gamedesign.MatchTypePvp)

				require.NoError(t, err)
				resp1, err := interactor.GetPlayerResponse(context.Background(), player1ID)
				require.NoError(t, err)
				assert.Equal(t, int64(50), resp1.Exp)
				resp2, err := interactor.GetPlayerResponse(context.Background(), player2ID)
				require.NoError(t, err)
				assert.Equal(t, int64(10), resp2.Exp)
			})

			t.Run("winner_numが2のとき、プレイヤー2に勝利時経験値を、プレイヤー1に敗北時経験値を付与する", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				player1ID := registerTestPlayer(t, "firebase-game-exp-4a")
				player2ID := registerTestPlayer(t, "firebase-game-exp-4b")

				err := interactor.AwardGameExp(context.Background(), player1ID, player2ID, 2, gamelogic.WinReasonBudgetZero, gamedesign.MatchTypePvp)

				require.NoError(t, err)
				resp1, err := interactor.GetPlayerResponse(context.Background(), player1ID)
				require.NoError(t, err)
				assert.Equal(t, int64(10), resp1.Exp)
				resp2, err := interactor.GetPlayerResponse(context.Background(), player2ID)
				require.NoError(t, err)
				assert.Equal(t, int64(50), resp2.Exp)
			})

			t.Run("match_typeがnpcのとき、プレイヤー2への経験値付与は行われない", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				player1ID := registerTestPlayer(t, "firebase-game-exp-5a")
				player2ID := registerTestPlayer(t, "firebase-game-exp-5b")

				err := interactor.AwardGameExp(context.Background(), player1ID, player2ID, 1, gamelogic.WinReasonBudgetZero, gamedesign.MatchTypeNpc)

				require.NoError(t, err)
				resp1, err := interactor.GetPlayerResponse(context.Background(), player1ID)
				require.NoError(t, err)
				assert.Equal(t, int64(50), resp1.Exp)
				resp2, err := interactor.GetPlayerResponse(context.Background(), player2ID)
				require.NoError(t, err)
				assert.Equal(t, int64(0), resp2.Exp)
			})
		})
	})
}

func TestPlayerInteractor_GetPlayerResponse(t *testing.T) {
	t.Run("PlayerInteractor", func(t *testing.T) {
		t.Run("GetPlayerResponse", func(t *testing.T) {
			t.Run("対象プレイヤーが存在しないとき、エラーを返す", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())

				_, err := interactor.GetPlayerResponse(context.Background(), uuid.NewString())

				require.Error(t, err)
			})

			t.Run("対象プレイヤーが存在するとき、プレイヤー情報を返す", func(t *testing.T) {
				interactor := newTestPlayerInteractor(t, validGameConfigValues())
				playerID := registerTestPlayer(t, "firebase-getresp-1")

				resp, err := interactor.GetPlayerResponse(context.Background(), playerID)

				require.NoError(t, err)
				assert.Equal(t, playerID, resp.PlayerID)
			})
		})
	})
}
