package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

func TestGetPlayerExpProgress(t *testing.T) {
	t.Run("[プレイヤー情報]レベルとレベル内の経験値進捗の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("ゲーム設定の経験値係数が100で累計経験値が450のとき", func(t *testing.T) {
				t.Run("取得すると応答のlevelは2になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					env.Store.SetProgression(playerID, 450)
					info, err := env.Player.GetPlayer(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(2), info.Level)
				})

				t.Run("取得すると応答のlevel_exp_currentは50になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					env.Store.SetProgression(playerID, 450)
					info, err := env.Player.GetPlayer(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(50), info.LevelProgress.LevelExpCurrent)
				})

				t.Run("取得すると応答のlevel_exp_requiredは500になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					env.Store.SetProgression(playerID, 450)
					info, err := env.Player.GetPlayer(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(500), info.LevelProgress.LevelExpRequired)
				})
			})

			t.Run("ゲーム設定の経験値係数が100で累計経験値が0のとき", func(t *testing.T) {
				t.Run("取得すると応答のlevelは1になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					info, err := env.Player.GetPlayer(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(1), info.Level)
				})

				t.Run("取得すると応答のlevel_exp_currentは0になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					info, err := env.Player.GetPlayer(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(0), info.LevelProgress.LevelExpCurrent)
				})

				t.Run("取得すると応答のlevel_exp_requiredは400になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					info, err := env.Player.GetPlayer(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(400), info.LevelProgress.LevelExpRequired)
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("ゲーム設定の経験値係数が0のとき、取得すると、経験値係数が1未満であることを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				env.Store.SetGameConfig("exp_formula_coefficient", 0)
				_, err := env.Player.GetPlayer(t.Context(), playerID)
				assert.ErrorIs(t, err, domain.ErrInvalidExpCoefficient)
			})

			t.Run("ゲーム設定に経験値係数が存在しないとき、取得するとゲーム設定が見つからないことを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				env.Store.RemoveGameConfig("exp_formula_coefficient")
				_, err := env.Player.GetPlayer(t.Context(), playerID)
				assert.ErrorIs(t, err, usecase.ErrGameConfigNotFound)
			})
		})
	})
}
