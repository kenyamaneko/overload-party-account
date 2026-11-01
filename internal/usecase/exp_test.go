package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

func TestAwardExp(t *testing.T) {
	t.Run("[経験値]累計経験値の加算", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("累計経験値が0のとき、経験値1を加算すると、加算したあとに取得した累計経験値は1になる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				require.NoError(t, env.Player.AwardExp(t.Context(), playerID, 1))
				assert.Equal(t, int64(1), usecasetest.ReadStoredExp(t, env, playerID))
			})

			t.Run("累計経験値が100のとき、経験値50を加算すると、加算したあとに取得した累計経験値は150になる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				env.Store.SetProgression(playerID, 100)
				require.NoError(t, env.Player.AwardExp(t.Context(), playerID, 50))
				assert.Equal(t, int64(150), usecasetest.ReadStoredExp(t, env, playerID))
			})

			t.Run("累計経験値が100のとき、経験値0を加算すると", func(t *testing.T) {
				t.Run("エラーにならない", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					env.Store.SetProgression(playerID, 100)
					assert.NoError(t, env.Player.AwardExp(t.Context(), playerID, 0))
				})

				t.Run("加算したあとに取得した累計経験値は100のままになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					env.Store.SetProgression(playerID, 100)
					require.NoError(t, env.Player.AwardExp(t.Context(), playerID, 0))
					assert.Equal(t, int64(100), usecasetest.ReadStoredExp(t, env, playerID))
				})
			})

			t.Run("ゲーム設定の経験値係数が100で累計経験値が0のとき、経験値450を加算したあとにプレイヤー情報を取得すると、応答のlevelは2になる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				require.NoError(t, env.Player.AwardExp(t.Context(), playerID, 450))
				info, err := env.Player.GetPlayer(t.Context(), playerID)
				require.NoError(t, err)
				assert.Equal(t, int64(2), info.Level)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("経験値-1を加算すると、加算量が不正であることを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				err := env.Player.AwardExp(t.Context(), playerID, -1)
				assert.ErrorIs(t, err, usecase.ErrInvalidExpGain)
			})
		})
	})
}
