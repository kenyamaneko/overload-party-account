package presenter_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/presenter"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

func TestBuildPlayerResponse(t *testing.T) {
	t.Run("BuildPlayerResponse", func(t *testing.T) {
		t.Run("Read Modelの各フィールドの値を、そのままPlayerResponseの対応フィールドに複写する", func(t *testing.T) {
			name := "テストプレイヤー"
			iconNo := int64(7)
			initialFaction := "SHE"
			premiumExpiresAt := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
			createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			updatedAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

			view := &domain.PlayerView{
				Player: domain.Player{
					PlayerID:         "player-1",
					FirebaseUID:      "firebase-1",
					Name:             &name,
					IsPremium:        true,
					EquippedIconNo:   &iconNo,
					OnboardingStatus: domain.OnboardingStatusCompleted,
					PremiumExpiresAt: &premiumExpiresAt,
					CreatedAt:        createdAt,
					UpdatedAt:        updatedAt,
				},
				Level:          1,
				Exp:            0,
				InitialFaction: &initialFaction,
			}

			resp, err := presenter.BuildPlayerResponse(view, 100)

			require.NoError(t, err)
			assert.Equal(t, "player-1", resp.PlayerID)
			assert.Equal(t, "firebase-1", resp.FirebaseUID)
			assert.Equal(t, &name, resp.Name)
			assert.Equal(t, true, resp.IsPremium)
			assert.Equal(t, &iconNo, resp.EquippedIconNo)
			assert.Equal(t, &initialFaction, resp.InitialFaction)
			assert.Equal(t, &premiumExpiresAt, resp.PremiumExpiresAt)
			assert.Equal(t, createdAt, resp.CreatedAt)
			assert.Equal(t, updatedAt, resp.UpdatedAt)
		})

		t.Run("Read Modelのオンボーディング状態の文字列を、そのままwire契約のOnboardingStatus型として設定する", func(t *testing.T) {
			view := &domain.PlayerView{
				Player: domain.Player{
					PlayerID:         "player-1",
					FirebaseUID:      "firebase-1",
					OnboardingStatus: domain.OnboardingStatusFactionSet,
					CreatedAt:        time.Now(),
					UpdatedAt:        time.Now(),
				},
				Level: 1,
				Exp:   0,
			}

			resp, err := presenter.BuildPlayerResponse(view, 100)

			require.NoError(t, err)
			assert.Equal(t, apiaccount.OnboardingStatusFactionSet, resp.OnboardingStatus)
		})

		t.Run("レベル進捗の計算結果を用いてLevelExpCurrentとLevelExpRequiredを算出する", func(t *testing.T) {
			// coeff=100, level=2 の開始閾値は400、次レベル(3)必要経験値は900
			view := &domain.PlayerView{
				Player: domain.Player{
					PlayerID:         "player-1",
					FirebaseUID:      "firebase-1",
					OnboardingStatus: domain.OnboardingStatusNotStarted,
					CreatedAt:        time.Now(),
					UpdatedAt:        time.Now(),
				},
				Level: 2,
				Exp:   450,
			}

			resp, err := presenter.BuildPlayerResponse(view, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(50), resp.LevelExpCurrent)
			assert.Equal(t, int64(500), resp.LevelExpRequired)
		})

		t.Run("レベルの不整合を検知してレベル進捗の計算がエラーを返すとき、エラーを返す", func(t *testing.T) {
			// level=2の開始閾値(coeff=100で400)未満のexpは不整合とみなされエラーになる
			view := &domain.PlayerView{
				Player: domain.Player{
					PlayerID:         "player-1",
					FirebaseUID:      "firebase-1",
					OnboardingStatus: domain.OnboardingStatusNotStarted,
					CreatedAt:        time.Now(),
					UpdatedAt:        time.Now(),
				},
				Level: 2,
				Exp:   0,
			}

			_, err := presenter.BuildPlayerResponse(view, 100)

			require.Error(t, err)
		})
	})
}

func TestBuildPlayerSettingsResponse(t *testing.T) {
	t.Run("BuildPlayerSettingsResponse", func(t *testing.T) {
		t.Run("PlayerSettingsの各フィールドの値を、そのままPlayerSettingsResponseの対応フィールドに複写する", func(t *testing.T) {
			updatedAt := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
			settings := &domain.PlayerSettings{
				PlayerID:    "player-1",
				Language:    "en",
				BgmVolume:   80,
				SeVolume:    30,
				PushEnabled: false,
				UpdatedAt:   updatedAt,
			}

			resp := presenter.BuildPlayerSettingsResponse(settings)

			assert.Equal(t, "player-1", resp.PlayerID)
			assert.Equal(t, "en", resp.Language)
			assert.Equal(t, int64(80), resp.BgmVolume)
			assert.Equal(t, int64(30), resp.SeVolume)
			assert.Equal(t, false, resp.PushEnabled)
			assert.Equal(t, updatedAt, resp.UpdatedAt)
		})
	})
}
