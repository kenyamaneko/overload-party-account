//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyFactionSet(t *testing.T) {
	t.Run("onboarding-faction-set の適用", func(t *testing.T) {
		t.Run("name 未確定 (NULL) でも initial_faction が反映され processed になる", func(t *testing.T) {
			// name はシナリオが入力時点で確定済みのため、faction-set 経路は initial_faction の
			// 反映と冪等ガードのみを担い、name には触れない。
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "", false) // name 未確定 (NULL) のプレイヤー

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			processed, err := svc.ApplyFactionSet(
				ctx,
				"44444444-4444-4444-4444-444444444444",
				"onboarding.faction-set",
				testPlayerID1,
				"SHE",
			)
			require.NoError(t, err)
			assert.True(t, processed)

			// faction だけ反映され、name は触られず NULL のまま。
			p, ferr := playerRepo.FindByID(ctx, testPlayerID1)
			require.NoError(t, ferr)
			assert.Nil(t, p.Name, "ApplyFactionSet は name を書かない")

			initial, ferr := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, ferr)
			require.NotNil(t, initial)
			assert.Equal(t, "SHE", *initial)

			factions, ferr := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, ferr)
			assert.ElementsMatch(t, []string{"SHE"}, factions)
		})
	})
}
