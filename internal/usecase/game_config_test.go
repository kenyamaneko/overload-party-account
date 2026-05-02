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

	wantErrContains := func(substr string) func(t *testing.T, err error) {
		return func(t *testing.T, err error) {
			require.Error(t, err)
			assert.Contains(t, err.Error(), substr)
		}
	}
	wantNoErr := func(t *testing.T, err error) {
		require.NoError(t, err)
	}

	tests := []struct {
		name   string
		mutate func(map[string]int64)
		assert func(t *testing.T, err error)
	}{
		{
			name:   "全キーが妥当値で投入されていれば nil",
			mutate: func(_ map[string]int64) {},
			assert: wantNoErr,
		},
		{
			name:   "free_daily_battle_limit が未投入ならエラー",
			mutate: func(m map[string]int64) { delete(m, configKeyFreeDailyBattleLimit) },
			assert: wantErrContains(configKeyFreeDailyBattleLimit),
		},
		{
			name:   "free_daily_battle_limit が 0 ならエラー (正の整数を要求)",
			mutate: func(m map[string]int64) { m[configKeyFreeDailyBattleLimit] = 0 },
			assert: wantErrContains(configKeyFreeDailyBattleLimit),
		},
		{
			name:   "free_daily_battle_limit が負ならエラー",
			mutate: func(m map[string]int64) { m[configKeyFreeDailyBattleLimit] = -1 },
			assert: wantErrContains(configKeyFreeDailyBattleLimit),
		},
		{
			name:   "exp_formula_coefficient が 0 ならエラー (レベル算出の前提を破る)",
			mutate: func(m map[string]int64) { m[ConfigKeyExpFormulaCoefficient] = 0 },
			assert: wantErrContains(ConfigKeyExpFormulaCoefficient),
		},
		{
			name:   "exp_win が未投入ならエラー (値 0 自体は許すがドキュメント存在は必須)",
			mutate: func(m map[string]int64) { delete(m, gameConfigKeyExpWin) },
			assert: wantErrContains(gameConfigKeyExpWin),
		},
		{
			name:   "exp_win が 0 でも通る (値 0 = 経験値なしは仕様上有効)",
			mutate: func(m map[string]int64) { m[gameConfigKeyExpWin] = 0 },
			assert: wantNoErr,
		},
		{
			name:   "exp_loss が未投入ならエラー",
			mutate: func(m map[string]int64) { delete(m, gameConfigKeyExpLoss) },
			assert: wantErrContains(gameConfigKeyExpLoss),
		},
		{
			name:   "exp_draw が未投入ならエラー",
			mutate: func(m map[string]int64) { delete(m, gameConfigKeyExpDraw) },
			assert: wantErrContains(gameConfigKeyExpDraw),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := validValues()
			tt.mutate(values)
			repo := newFakeGameConfigRepo(values)
			tt.assert(t, ValidateGameConfig(ctx, repo))
		})
	}
}
