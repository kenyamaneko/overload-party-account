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

// BuildPlayerResponse はフィールド射影を担う。レベル進捗計算は domain.ComputeExpProgress
// 側の単体テストで網羅し、ここでは「presenter が domain の結果をそのまま wire に詰めている」
// ことだけ確認する。
func TestBuildPlayerResponse(t *testing.T) {
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
	const coeff = int64(100)
	wantProgress, err := domain.ComputeExpProgress(view.Level, view.Exp, coeff)
	assert.NoError(t, err)

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
	assert.Equal(t, wantProgress.LevelExpCurrent, got.LevelExpCurrent)
	assert.Equal(t, wantProgress.LevelExpRequired, got.LevelExpRequired)
}

func TestBuildPlayerSettingsResponse(t *testing.T) {
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
}
