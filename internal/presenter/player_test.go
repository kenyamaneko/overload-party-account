package presenter_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/presenter"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

func strPtr(s string) *string { return &s }
func i64Ptr(n int64) *int64   { return &n }

func TestBuildPlayerResponse(t *testing.T) {
	t.Run("プレイヤー応答の組み立て", func(t *testing.T) {
		t.Run("PlayerView の各フィールドと進捗が wire 応答に射影される", func(t *testing.T) {
			// レベル進捗の算出そのものは domain.ComputeExpProgress の単体テストで網羅し、
			// ここでは presenter が domain の結果をそのまま wire に詰めていることだけ確認する。
			created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			updated := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
			expires := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
			faction := "SHE"

			view := &domain.PlayerView{
				Player: domain.Player{
					PlayerID:         "p1",
					FirebaseUID:      "fb1",
					Name:             strPtr("Kenya"),
					IsPremium:        true,
					EquippedIconNo:   i64Ptr(3),
					OnboardingStatus: domain.OnboardingStatusCompleted,
					PremiumExpiresAt: &expires,
					CreatedAt:        created,
					UpdatedAt:        updated,
				},
				Level:          2,
				Exp:            500,
				InitialFaction: &faction,
			}
			const (
				coeff                = int64(100)
				wantLevelExpCurrent  = int64(100)
				wantLevelExpRequired = int64(500)
			)

			got, err := presenter.BuildPlayerResponse(view, coeff)
			assert.NoError(t, err)

			assert.Equal(t, "p1", got.PlayerID)
			assert.Equal(t, "fb1", got.FirebaseUID)
			assert.Equal(t, strPtr("Kenya"), got.Name)
			assert.Equal(t, view.Level, got.Level)
			assert.Equal(t, view.Exp, got.Exp)
			assert.True(t, got.IsPremium)
			assert.Equal(t, i64Ptr(3), got.EquippedIconNo)
			assert.Equal(t, &faction, got.InitialFaction)
			assert.Equal(t, &expires, got.PremiumExpiresAt)
			assert.Equal(t, apiaccount.OnboardingStatus(domain.OnboardingStatusCompleted), got.OnboardingStatus)
			assert.Equal(t, created, got.CreatedAt)
			assert.Equal(t, updated, got.UpdatedAt)
			assert.Equal(t, wantLevelExpCurrent, got.LevelExpCurrent)
			assert.Equal(t, wantLevelExpRequired, got.LevelExpRequired)
		})
	})
}

func TestBuildPlayerSettingsResponse(t *testing.T) {
	t.Run("プレイヤー設定応答の組み立て", func(t *testing.T) {
		t.Run("PlayerSettings の各フィールドが wire 応答に射影される", func(t *testing.T) {
			updated := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
			in := &domain.PlayerSettings{
				PlayerID:    "p1",
				Language:    "ja",
				BgmVolume:   80,
				SeVolume:    60,
				PushEnabled: true,
				UpdatedAt:   updated,
			}

			got := presenter.BuildPlayerSettingsResponse(in)

			assert.Equal(t, "p1", got.PlayerID)
			assert.Equal(t, "ja", got.Language)
			assert.Equal(t, int64(80), got.BgmVolume)
			assert.Equal(t, int64(60), got.SeVolume)
			assert.True(t, got.PushEnabled)
			assert.Equal(t, updated, got.UpdatedAt)
		})
	})
}
