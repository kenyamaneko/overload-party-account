//go:build integration

package usecase_test

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

// createPlayerWithoutSettings は players / player_progression のみを作成し、
// player_settings 行を意図的に作らないことで「設定行が存在しない」状態を再現する。
// この状態は正規の登録フロー (AuthInteractor.Register) では発生し得ないため、
// リポジトリを直接使って前提を組み立てる。
func createPlayerWithoutSettings(t *testing.T) string {
	t.Helper()
	playerID := uuid.NewString()
	now := time.Now().UTC()
	playerRepo := postgres.NewPlayerRepository(sharedPg.Pool)
	err := playerRepo.Create(context.Background(), &domain.Player{
		PlayerID:         playerID,
		FirebaseUID:      "firebase-no-settings-" + playerID,
		IsPremium:        false,
		OnboardingStatus: domain.OnboardingStatusNotStarted,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, &domain.PlayerProgression{
		PlayerID:  playerID,
		Level:     1,
		Exp:       0,
		UpdatedAt: now,
	})
	require.NoError(t, err)
	return playerID
}

func TestPlayerSettingsInteractor_Get(t *testing.T) {
	t.Run("PlayerSettingsInteractor", func(t *testing.T) {
		t.Run("Get", func(t *testing.T) {
			t.Run("対象プレイヤーの設定行が存在しないとき、エラーを返す", func(t *testing.T) {
				interactor := newTestPlayerSettingsInteractor(t)
				playerID := createPlayerWithoutSettings(t)

				_, err := interactor.Get(context.Background(), playerID)

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("設定行が存在するとき、設定情報を返す", func(t *testing.T) {
				interactor := newTestPlayerSettingsInteractor(t)
				playerID := registerTestPlayer(t, "firebase-settings-get-1")

				resp, err := interactor.Get(context.Background(), playerID)

				require.NoError(t, err)
				assert.Equal(t, playerID, resp.PlayerID)
				assert.Equal(t, "ja", resp.Language)
			})
		})
	})
}

func TestPlayerSettingsInteractor_Update(t *testing.T) {
	t.Run("PlayerSettingsInteractor", func(t *testing.T) {
		t.Run("Updateは、patchで指定された(nilでない)フィールドのみが更新される", func(t *testing.T) {
			interactor := newTestPlayerSettingsInteractor(t)
			playerID := registerTestPlayer(t, "firebase-settings-update-1")
			newLanguage := "en"

			err := interactor.Update(context.Background(), playerID, &port.PlayerSettingsPatch{Language: &newLanguage})

			require.NoError(t, err)
			resp, err := interactor.Get(context.Background(), playerID)
			require.NoError(t, err)
			assert.Equal(t, "en", resp.Language)
			assert.Equal(t, int64(50), resp.BgmVolume)
			assert.Equal(t, int64(50), resp.SeVolume)
			assert.True(t, resp.PushEnabled)
		})
	})
}
