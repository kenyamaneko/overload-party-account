package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

func validateWithConfigValue(t *testing.T, key string, value int64) error {
	t.Helper()
	store := usecasetest.NewMemoryStore()
	store.SetGameConfig(key, value)
	return usecase.ValidateGameConfig(t.Context(), store.GameConfigRepo())
}

func validateWithoutConfigKey(t *testing.T, key string) error {
	t.Helper()
	store := usecasetest.NewMemoryStore()
	store.RemoveGameConfig(key)
	return usecase.ValidateGameConfig(t.Context(), store.GameConfigRepo())
}

func TestValidateGameConfig(t *testing.T) {
	t.Run("[ゲーム設定]ゲーム設定値の起動時検証", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("検証するとき", func(t *testing.T) {
				tests := []struct {
					name      string
					overrides map[string]int64
				}{
					{"free_daily_battle_limitが1のとき、エラーにならない", map[string]int64{"free_daily_battle_limit": 1}},
					{"exp_formula_coefficientが1のとき、エラーにならない", map[string]int64{"exp_formula_coefficient": 1}},
					{"exp_winが0のとき、エラーにならない", map[string]int64{"exp_win": 0}},
					{"exp_lossが0のとき、エラーにならない", map[string]int64{"exp_loss": 0}},
					{"exp_drawが0のとき、エラーにならない", map[string]int64{"exp_draw": 0}},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						store := usecasetest.NewMemoryStore()
						for key, value := range tt.overrides {
							store.SetGameConfig(key, value)
						}
						assert.NoError(t, usecase.ValidateGameConfig(t.Context(), store.GameConfigRepo()))
					})
				}
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("正の整数が必要なキーが1未満のとき", func(t *testing.T) {
				t.Run("free_daily_battle_limitが0のとき、検証するとfree_daily_battle_limitが1未満であることを示すエラーになる", func(t *testing.T) {
					err := validateWithConfigValue(t, "free_daily_battle_limit", 0)
					require.ErrorIs(t, err, usecase.ErrGameConfigBelowMinimum)
					assert.ErrorContains(t, err, "free_daily_battle_limit")
				})

				t.Run("exp_formula_coefficientが0のとき、検証するとexp_formula_coefficientが1未満であることを示すエラーになる", func(t *testing.T) {
					err := validateWithConfigValue(t, "exp_formula_coefficient", 0)
					require.ErrorIs(t, err, usecase.ErrGameConfigBelowMinimum)
					assert.ErrorContains(t, err, "exp_formula_coefficient")
				})
			})

			t.Run("キーが存在しないとき", func(t *testing.T) {
				t.Run("free_daily_battle_limitが存在しないとき、検証するとfree_daily_battle_limitを示す設定が見つからないエラーになる", func(t *testing.T) {
					err := validateWithoutConfigKey(t, "free_daily_battle_limit")
					require.ErrorIs(t, err, port.ErrNotFound)
					assert.ErrorContains(t, err, "free_daily_battle_limit")
				})

				t.Run("exp_formula_coefficientが存在しないとき、検証するとexp_formula_coefficientを示す設定が見つからないエラーになる", func(t *testing.T) {
					err := validateWithoutConfigKey(t, "exp_formula_coefficient")
					require.ErrorIs(t, err, port.ErrNotFound)
					assert.ErrorContains(t, err, "exp_formula_coefficient")
				})

				t.Run("exp_winが存在しないとき、検証するとexp_winを示す設定が見つからないエラーになる", func(t *testing.T) {
					err := validateWithoutConfigKey(t, "exp_win")
					require.ErrorIs(t, err, port.ErrNotFound)
					assert.ErrorContains(t, err, "exp_win")
				})

				t.Run("exp_lossが存在しないとき、検証するとexp_lossを示す設定が見つからないエラーになる", func(t *testing.T) {
					err := validateWithoutConfigKey(t, "exp_loss")
					require.ErrorIs(t, err, port.ErrNotFound)
					assert.ErrorContains(t, err, "exp_loss")
				})

				t.Run("exp_drawが存在しないとき、検証するとexp_drawを示す設定が見つからないエラーになる", func(t *testing.T) {
					err := validateWithoutConfigKey(t, "exp_draw")
					require.ErrorIs(t, err, port.ErrNotFound)
					assert.ErrorContains(t, err, "exp_draw")
				})
			})

			t.Run("経験値の設定値が0未満のとき", func(t *testing.T) {
				t.Run("exp_winが-1のとき、検証するとexp_winが0未満であることを示すエラーになる", func(t *testing.T) {
					err := validateWithConfigValue(t, "exp_win", -1)
					require.ErrorIs(t, err, usecase.ErrGameConfigBelowMinimum)
					assert.ErrorContains(t, err, "exp_win")
				})

				t.Run("exp_lossが-1のとき、検証するとexp_lossが0未満であることを示すエラーになる", func(t *testing.T) {
					err := validateWithConfigValue(t, "exp_loss", -1)
					require.ErrorIs(t, err, usecase.ErrGameConfigBelowMinimum)
					assert.ErrorContains(t, err, "exp_loss")
				})

				t.Run("exp_drawが-1のとき、検証するとexp_drawが0未満であることを示すエラーになる", func(t *testing.T) {
					err := validateWithConfigValue(t, "exp_draw", -1)
					require.ErrorIs(t, err, usecase.ErrGameConfigBelowMinimum)
					assert.ErrorContains(t, err, "exp_draw")
				})
			})

			t.Run("ゲーム設定の保存先が接続エラーを返すとき、検証するとfree_daily_battle_limitを示す接続エラーを原因とするエラーになる", func(t *testing.T) {
				store := usecasetest.NewMemoryStore()
				store.FailGameConfigLookupWith(usecasetest.ErrConnection)
				err := usecase.ValidateGameConfig(t.Context(), store.GameConfigRepo())
				require.ErrorIs(t, err, usecasetest.ErrConnection)
				assert.ErrorContains(t, err, "free_daily_battle_limit")
			})
		})
	})
}
