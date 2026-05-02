//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ApplyFactionSet は表示名を扱わない契約: シナリオは入力時点で
// PUT /players/:id/name を呼んで account に確定済みのため、onboarding-faction-set 経路では
// initial_faction の反映と processed_events の冪等ガードのみを担う。
// name が NULL のままでも faction の付与には支障がないことをここで固定する。
func TestOnboardingInteractor_ApplyFactionSet_NameIndependent(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "", false) // name 未確定 (NULL) のプレイヤー

	playerRepo, _, factionRepo, _, tx := newRealRepos()
	eventRepo := newProcessedEventRepo()
	svc := NewOnboardingInteractor(playerRepo, factionRepo, eventRepo, tx)

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
}
