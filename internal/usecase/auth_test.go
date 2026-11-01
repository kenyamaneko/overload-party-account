package usecase_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port/porttest"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

const issuedPlayerID = "7b0c5a52-3f7e-4d2b-9d5e-0a1b2c3d4e5f"

func newUnitEnvWithIssuedPlayerID(t *testing.T) *usecasetest.UnitEnv {
	t.Helper()
	return usecasetest.NewUnitEnvWithPlayerIDGenerator(t, porttest.NewFixedPlayerIDGenerator(issuedPlayerID))
}

func TestRegister(t *testing.T) {
	t.Run("[プレイヤー登録]新規登録", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("未登録のfirebase_uidで登録すると", func(t *testing.T) {
				t.Run("応答のlevelは1になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					assert.Equal(t, int64(1), info.Level)
				})

				t.Run("応答のexpは0になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					assert.Equal(t, int64(0), info.View.Exp)
				})

				t.Run("応答のis_premiumはfalseになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					assert.False(t, info.View.Player.IsPremium)
				})

				t.Run("応答にnameは含まれない", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					assert.Nil(t, info.View.Player.Name)
				})

				t.Run("応答にinitial_factionは含まれない", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					assert.Nil(t, info.View.InitialFaction)
				})

				t.Run("応答のonboarding_statusはnot_startedになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					assert.Equal(t, "not_started", info.View.Player.OnboardingStatus)
				})

				t.Run("応答のcreated_atは登録を実行した時刻になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					assert.Equal(t, time.Date(2026, 1, 10, 12, 0, 0, 0, usecasetest.JapanTimeZone).UTC(), info.View.Player.CreatedAt.UTC())
				})

				t.Run("応答のupdated_atは登録を実行した時刻になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					assert.Equal(t, time.Date(2026, 1, 10, 12, 0, 0, 0, usecasetest.JapanTimeZone).UTC(), info.View.Player.UpdatedAt.UTC())
				})
			})

			t.Run("未登録のfirebase_uidで、採番されるプレイヤーIDが7b0c5a52-3f7e-4d2b-9d5e-0a1b2c3d4e5fのとき、登録すると応答のplayer_idは7b0c5a52-3f7e-4d2b-9d5e-0a1b2c3d4e5fになる", func(t *testing.T) {
				env := newUnitEnvWithIssuedPlayerID(t)
				info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
				require.NoError(t, err)
				assert.Equal(t, "7b0c5a52-3f7e-4d2b-9d5e-0a1b2c3d4e5f", info.View.Player.PlayerID)
			})

			t.Run("ゲーム設定の経験値係数が100のとき、登録すると応答のlevel_exp_requiredは400になる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
				require.NoError(t, err)
				assert.Equal(t, int64(400), info.LevelProgress.LevelExpRequired)
			})

			t.Run("登録したあとにプレイヤー設定を取得すると", func(t *testing.T) {
				t.Run("言語はjaになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					settings, err := env.Store.PlayerSettingsRepo().GetPlayerSettings(t.Context(), info.View.Player.PlayerID)
					require.NoError(t, err)
					assert.Equal(t, "ja", settings.Language)
				})

				t.Run("bgm_volumeは50になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					settings, err := env.Store.PlayerSettingsRepo().GetPlayerSettings(t.Context(), info.View.Player.PlayerID)
					require.NoError(t, err)
					assert.Equal(t, int64(50), settings.BgmVolume)
				})

				t.Run("se_volumeは50になる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					settings, err := env.Store.PlayerSettingsRepo().GetPlayerSettings(t.Context(), info.View.Player.PlayerID)
					require.NoError(t, err)
					assert.Equal(t, int64(50), settings.SeVolume)
				})

				t.Run("push_enabledはtrueになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					settings, err := env.Store.PlayerSettingsRepo().GetPlayerSettings(t.Context(), info.View.Player.PlayerID)
					require.NoError(t, err)
					assert.True(t, settings.IsPushEnabled)
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("登録済みのfirebase_uidで登録すると、二重登録を示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				_, err := env.Auth.Register(t.Context(), "firebase-uid-1")
				require.NoError(t, err)
				_, err = env.Auth.Register(t.Context(), "firebase-uid-1")
				assert.ErrorIs(t, err, usecase.ErrPlayerAlreadyRegistered)
			})

			t.Run("firebase_uidがプレイヤーID7b0c5a52-3f7e-4d2b-9d5e-0a1b2c3d4e5fで登録済みのとき、登録に失敗したあとにログインすると、player_idは7b0c5a52-3f7e-4d2b-9d5e-0a1b2c3d4e5fになる", func(t *testing.T) {
				env := newUnitEnvWithIssuedPlayerID(t)
				_, err := env.Auth.Register(t.Context(), "firebase-uid-1")
				require.NoError(t, err)
				_, err = env.Auth.Register(t.Context(), "firebase-uid-1")
				require.Error(t, err)
				loggedIn, err := env.Auth.Login(t.Context(), "firebase-uid-1")
				require.NoError(t, err)
				assert.Equal(t, "7b0c5a52-3f7e-4d2b-9d5e-0a1b2c3d4e5f", loggedIn.View.Player.PlayerID)
			})

			t.Run("登録の保存がデータベース接続エラーで失敗したとき、登録すると接続エラーを原因とするエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				env.Store.FailPlayerCreateWith(usecasetest.ErrConnection)
				_, err := env.Auth.Register(t.Context(), "firebase-uid-1")
				assert.ErrorIs(t, err, usecasetest.ErrConnection)
			})

			t.Run("ゲーム設定に経験値係数が存在しないとき", func(t *testing.T) {
				t.Run("登録すると、ゲーム設定が見つからないことを示すエラーになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					env.Store.RemoveGameConfig("exp_formula_coefficient")
					_, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					assert.ErrorIs(t, err, usecase.ErrGameConfigNotFound)
				})

				t.Run("採番されるプレイヤーIDが7b0c5a52-3f7e-4d2b-9d5e-0a1b2c3d4e5fのとき、登録に失敗したあと、経験値係数を100に設定して同じfirebase_uidで登録し直すと、応答のplayer_idは7b0c5a52-3f7e-4d2b-9d5e-0a1b2c3d4e5fになる", func(t *testing.T) {
					env := newUnitEnvWithIssuedPlayerID(t)
					env.Store.RemoveGameConfig("exp_formula_coefficient")
					_, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.Error(t, err)
					env.Store.SetGameConfig("exp_formula_coefficient", 100)
					info, err := env.Auth.Register(t.Context(), "firebase-uid-1")
					require.NoError(t, err)
					assert.Equal(t, "7b0c5a52-3f7e-4d2b-9d5e-0a1b2c3d4e5f", info.View.Player.PlayerID)
				})
			})

			t.Run("ゲーム設定の経験値係数が0のとき、登録すると、経験値係数が1未満であることを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				env.Store.SetGameConfig("exp_formula_coefficient", 0)
				_, err := env.Auth.Register(t.Context(), "firebase-uid-1")
				assert.ErrorIs(t, err, domain.ErrInvalidExpCoefficient)
			})
		})
	})
}
