//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validValues は ValidateGameConfig が通る最小セット。テストケースが mutate で破壊する。
func validValues() map[string]int64 {
	return map[string]int64{
		configKeyFreeDailyBattleLimit:  10,
		ConfigKeyExpFormulaCoefficient: 60,
		gameConfigKeyExpWin:            40,
		gameConfigKeyExpLoss:           20,
		gameConfigKeyExpDraw:           30,
	}
}

func TestValidateGameConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("ゲーム設定値のバリデーション", func(t *testing.T) {
		validCases := []struct {
			name   string
			mutate func(map[string]int64)
		}{
			{
				name:   "全キーが妥当値で投入されているとき、エラーにならない",
				mutate: func(_ map[string]int64) {},
			},
			{
				name:   "exp_win が 0 のとき、エラーにならない (値 0 = 経験値なしは仕様上有効)",
				mutate: func(m map[string]int64) { m[gameConfigKeyExpWin] = 0 },
			},
		}
		for _, tc := range validCases {
			t.Run(tc.name, func(t *testing.T) {
				values := validValues()
				tc.mutate(values)
				repo := newFakeGameConfigRepo(values)
				require.NoError(t, ValidateGameConfig(ctx, repo))
			})
		}

		invalidCases := []struct {
			name         string
			mutate       func(map[string]int64)
			wantContains []string
		}{
			{
				name:         "free_daily_battle_limit が未投入のとき、エラーになる",
				mutate:       func(m map[string]int64) { delete(m, configKeyFreeDailyBattleLimit) },
				wantContains: []string{configKeyFreeDailyBattleLimit},
			},
			{
				name:         "free_daily_battle_limit が 0 のとき、エラーになる (正の整数を要求)",
				mutate:       func(m map[string]int64) { m[configKeyFreeDailyBattleLimit] = 0 },
				wantContains: []string{configKeyFreeDailyBattleLimit},
			},
			{
				name:         "free_daily_battle_limit が -1 のとき、エラーになる",
				mutate:       func(m map[string]int64) { m[configKeyFreeDailyBattleLimit] = -1 },
				wantContains: []string{configKeyFreeDailyBattleLimit},
			},
			{
				name:         "exp_formula_coefficient が 0 のとき、エラーになる (レベル算出の前提を破る)",
				mutate:       func(m map[string]int64) { m[ConfigKeyExpFormulaCoefficient] = 0 },
				wantContains: []string{ConfigKeyExpFormulaCoefficient},
			},
			{
				name:         "exp_win が未投入のとき、エラーになる (値 0 は許すが定義の存在は必須)",
				mutate:       func(m map[string]int64) { delete(m, gameConfigKeyExpWin) },
				wantContains: []string{gameConfigKeyExpWin},
			},
			{
				name:         "exp_loss が未投入のとき、エラーになる",
				mutate:       func(m map[string]int64) { delete(m, gameConfigKeyExpLoss) },
				wantContains: []string{gameConfigKeyExpLoss},
			},
			{
				name:         "exp_draw が未投入のとき、エラーになる",
				mutate:       func(m map[string]int64) { delete(m, gameConfigKeyExpDraw) },
				wantContains: []string{gameConfigKeyExpDraw},
			},
		}
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				values := validValues()
				tc.mutate(values)
				repo := newFakeGameConfigRepo(values)

				err := ValidateGameConfig(ctx, repo)

				require.Error(t, err)
				for _, substr := range tc.wantContains {
					assert.Contains(t, err.Error(), substr)
				}
			})
		}
	})
}
