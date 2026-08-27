//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

func createTestPlayerSettings(t *testing.T, playerID string) {
	t.Helper()
	repo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
	err := repo.Insert(context.Background(), &domain.PlayerSettings{
		PlayerID:    playerID,
		Language:    "ja",
		BgmVolume:   50,
		SeVolume:    50,
		PushEnabled: true,
		UpdatedAt:   time.Now().UTC(),
	})
	require.NoError(t, err)
}

func TestPlayerSettingsRepository_Get(t *testing.T) {
	t.Run("[PlayerSettingsRepository]プレイヤー設定の永続化", func(t *testing.T) {
		t.Run("Get", func(t *testing.T) {
			t.Run("存在しないplayer_idを指定したとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)

				_, err := repo.Get(context.Background(), uuid.NewString())

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("設定行が存在するとき、設定を返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				createTestPlayerSettings(t, player.PlayerID)
				repo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)

				settings, err := repo.Get(context.Background(), player.PlayerID)

				require.NoError(t, err)
				assert.Equal(t, "ja", settings.Language)
			})
		})
	})
}

func TestPlayerSettingsRepository_UpdatePartial(t *testing.T) {
	t.Run("[PlayerSettingsRepository]プレイヤー設定の永続化", func(t *testing.T) {
		t.Run("UpdatePartial", func(t *testing.T) {
			t.Run("存在しないplayer_idを指定したとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
				lang := "en"

				err := repo.UpdatePartial(context.Background(), uuid.NewString(), &port.PlayerSettingsPatch{Language: &lang})

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("更新内容で値を指定しなかったフィールドは変更されず既存値が保持され、値を指定したフィールドは新しい値に置き換わる", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				createTestPlayerSettings(t, player.PlayerID)
				repo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
				newBgm := int64(80)

				err := repo.UpdatePartial(context.Background(), player.PlayerID, &port.PlayerSettingsPatch{BgmVolume: &newBgm})

				require.NoError(t, err)
				settings, err := repo.Get(context.Background(), player.PlayerID)
				require.NoError(t, err)
				assert.Equal(t, int64(80), settings.BgmVolume)
				assert.Equal(t, "ja", settings.Language)
				assert.Equal(t, int64(50), settings.SeVolume)
				assert.True(t, settings.PushEnabled)
			})
		})
	})
}
