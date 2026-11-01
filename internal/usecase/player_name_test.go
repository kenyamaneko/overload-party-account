package usecase_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

func TestUpdateName(t *testing.T) {
	t.Run("[表示名]表示名の変更", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("表示名がゆうしゃのとき、変更すると応答のnameはゆうしゃになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				info, err := env.Player.UpdateName(t.Context(), playerID, "ゆうしゃ")
				require.NoError(t, err)
				assert.Equal(t, usecasetest.ToPointer("ゆうしゃ"), info.View.Player.Name)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("空文字の表示名に変更すると、表示名が不正であることを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				_, err := env.Player.UpdateName(t.Context(), playerID, "")
				assert.ErrorIs(t, err, domain.ErrInvalidName)
			})

			t.Run("表示名がゆうしゃのプレイヤーを空文字の表示名に変更しようとして失敗したあとにプレイヤー情報を取得すると、nameはゆうしゃのままになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				_, err := env.Player.UpdateName(t.Context(), playerID, "ゆうしゃ")
				require.NoError(t, err)
				_, err = env.Player.UpdateName(t.Context(), playerID, "")
				require.Error(t, err)
				info, err := env.Player.GetPlayer(t.Context(), playerID)
				require.NoError(t, err)
				assert.Equal(t, usecasetest.ToPointer("ゆうしゃ"), info.View.Player.Name)
			})
		})
	})
}

func TestValidateNameForOnboarding(t *testing.T) {
	t.Run("[表示名]オンボーディング時の表示名の検証", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("登録済みのプレイヤーに有効な表示名ゆうしゃを検証すると", func(t *testing.T) {
				t.Run("エラーにならない", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					err := env.Player.ValidateNameForOnboarding(t.Context(), playerID, "ゆうしゃ")
					assert.NoError(t, err)
				})

				t.Run("検証したあとにプレイヤー情報を取得すると、nameは含まれない", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					require.NoError(t, env.Player.ValidateNameForOnboarding(t.Context(), playerID, "ゆうしゃ"))
					info, err := env.Player.GetPlayer(t.Context(), playerID)
					require.NoError(t, err)
					assert.Nil(t, info.View.Player.Name)
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("登録済みのプレイヤーに空文字の表示名を検証すると、表示名が不正であることを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				err := env.Player.ValidateNameForOnboarding(t.Context(), playerID, "")
				assert.ErrorIs(t, err, domain.ErrInvalidName)
			})

			t.Run("未登録のプレイヤーに有効な表示名ゆうしゃを検証すると、プレイヤーが存在しないことを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				err := env.Player.ValidateNameForOnboarding(t.Context(), uuid.NewString(), "ゆうしゃ")
				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		})
	})
}
