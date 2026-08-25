//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

// createTestPlayer は players / player_progression 行を1組作成し、作成した Player を返す。
// 子テーブル (player_factions / player_settings / player_daily_battle 等) を対象にする
// テストの前提として、外部キー制約を満たす親行を用意するために使う。
func createTestPlayer(t *testing.T) *domain.Player {
	t.Helper()
	now := time.Now().UTC()
	player := &domain.Player{
		PlayerID:         uuid.NewString(),
		FirebaseUID:      "firebase-" + uuid.NewString(),
		IsPremium:        false,
		OnboardingStatus: domain.OnboardingStatusNotStarted,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	progression := &domain.PlayerProgression{
		PlayerID:  player.PlayerID,
		Level:     1,
		Exp:       0,
		UpdatedAt: now,
	}
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	require.NoError(t, repo.Create(context.Background(), player, progression))
	return player
}
