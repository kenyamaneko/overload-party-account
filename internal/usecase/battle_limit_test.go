package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

func incrementBattleCount(t *testing.T, player *usecase.PlayerInteractor, playerID string, times int) {
	t.Helper()
	for range times {
		require.NoError(t, player.IncrementBattleCount(t.Context(), playerID))
	}
}

func TestGetBattleLimit(t *testing.T) {
	t.Run("[バトル回数制限]1日のバトル回数の照会", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("照会するとき", func(t *testing.T) {
				tests := []struct {
					name           string
					prepare        func(env *usecasetest.UnitEnv, playerID string)
					incrementCount int
				}{
					{"無料プレイヤーで当日のバトル記録が無いとき、can_battleはtrueになる", func(*usecasetest.UnitEnv, string) {}, 0},
					{"無料プレイヤーで当日のバトル回数が設定した上限より1回少ないとき、can_battleはtrueになる", func(*usecasetest.UnitEnv, string) {}, 2},
					{"プレミアム中のプレイヤーで当日のバトル回数が無料プレイヤーの設定した上限と同じとき、can_battleはtrueになる", func(env *usecasetest.UnitEnv, playerID string) { env.Store.SetPremium(playerID) }, 3},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						env := usecasetest.NewUnitEnv(t)
						playerID := usecasetest.RegisterPlayer(t, env.Auth)
						tt.prepare(env, playerID)
						incrementBattleCount(t, env.Player, playerID, tt.incrementCount)
						limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
						require.NoError(t, err)
						assert.True(t, limit.CanBattle)
					})
				}
			})

			t.Run("無料プレイヤーのとき", func(t *testing.T) {
				t.Run("当日のバトル記録が無いとき、照会するとdaily_battle_countは0になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(0), limit.DailyBattleCount)
				})

				t.Run("当日に2回加算したとき、照会するとdaily_battle_countは2になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					incrementBattleCount(t, env.Player, playerID, 2)
					limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(2), limit.DailyBattleCount)
				})

				t.Run("無料プレイヤーの1日の上限が3回に設定されているとき、照会するとdaily_battle_limitは3になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(3), limit.DailyBattleLimit)
				})

				t.Run("当日のバトル回数が設定した上限と同じとき、照会するとcan_battleはfalseになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					incrementBattleCount(t, env.Player, playerID, 3)
					limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
					require.NoError(t, err)
					assert.False(t, limit.CanBattle)
				})
			})

			t.Run("プレミアム中のプレイヤーのとき", func(t *testing.T) {
				t.Run("照会するとdaily_battle_limitは-1になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					env.Store.SetPremium(playerID)
					limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(-1), limit.DailyBattleLimit)
				})

				t.Run("当日のバトル回数が無料プレイヤーの設定した上限と同じとき、照会するとdaily_battle_countは設定した上限と同じ値になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					env.Store.SetPremium(playerID)
					incrementBattleCount(t, env.Player, playerID, 3)
					limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(3), limit.DailyBattleCount)
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("無料プレイヤーでゲーム設定に無料プレイヤーの1日の上限が存在しないとき、照会するとゲーム設定が見つからないことを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				env.Store.RemoveGameConfig("free_daily_battle_limit")
				_, err := env.Player.GetBattleLimit(t.Context(), playerID)
				assert.ErrorIs(t, err, usecase.ErrGameConfigNotFound)
			})
		})
	})
}

func TestIncrementBattleCount(t *testing.T) {
	t.Run("[バトル回数制限]1日のバトル回数の加算", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("無料プレイヤーのとき", func(t *testing.T) {
				t.Run("当日のバトル回数が0のとき、加算すると照会したdaily_battle_countは1になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					require.NoError(t, env.Player.IncrementBattleCount(t.Context(), playerID))
					limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(1), limit.DailyBattleCount)
				})

				t.Run("当日のバトル回数が設定した上限より1回少ないとき、加算すると照会したdaily_battle_countは設定した上限と同じ値になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					incrementBattleCount(t, env.Player, playerID, 2)
					require.NoError(t, env.Player.IncrementBattleCount(t.Context(), playerID))
					limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(3), limit.DailyBattleCount)
				})
			})

			t.Run("プレミアム中のプレイヤーで当日のバトル回数が無料プレイヤーの設定した上限と同じとき、加算すると照会したdaily_battle_countは設定した上限より1大きい値になる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				env.Store.SetPremium(playerID)
				incrementBattleCount(t, env.Player, playerID, 3)
				require.NoError(t, env.Player.IncrementBattleCount(t.Context(), playerID))
				limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
				require.NoError(t, err)
				assert.Equal(t, int64(4), limit.DailyBattleCount)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("無料プレイヤーで当日のバトル回数が設定した上限と同じとき、加算すると、1日のバトル回数が上限に達していることを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				incrementBattleCount(t, env.Player, playerID, 3)
				err := env.Player.IncrementBattleCount(t.Context(), playerID)
				assert.ErrorIs(t, err, usecase.ErrBattleLimitExceeded)
			})

			t.Run("無料プレイヤーでゲーム設定に無料プレイヤーの1日の上限が存在しないとき、加算するとゲーム設定が見つからないことを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				env.Store.RemoveGameConfig("free_daily_battle_limit")
				err := env.Player.IncrementBattleCount(t.Context(), playerID)
				assert.ErrorIs(t, err, usecase.ErrGameConfigNotFound)
			})
		})
	})
}
