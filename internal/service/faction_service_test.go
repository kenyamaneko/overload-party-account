package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"

	"github.com/kenyamaneko/overload-party-account/internal/repository"
)

func newFactionTestEnv(t *testing.T, playerID string) (*FactionService, *repository.MockPlayerRepository, *repository.MockFactionRepository) {
	t.Helper()
	playerRepo := repository.NewMockPlayerRepository()
	factionRepo := repository.NewMockFactionRepository()
	svc := NewFactionService(playerRepo, factionRepo, &repository.MockTxRunner{})

	now := time.Now()
	player := &apiaccount.Player{
		PlayerID:    playerID,
		FirebaseUID: "uid-" + playerID,
		Username:    "tester",
		Level:       1,
		Exp:         0,
		IsPremium:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	dailyBattle := &apiaccount.PlayerDailyBattle{
		PlayerID:         playerID,
		DailyBattleCount: 0,
		LastResetDate:    civil.DateOf(now.UTC()),
	}
	require.NoError(t, playerRepo.Create(context.Background(), player, dailyBattle))
	return svc, playerRepo, factionRepo
}

func TestSelectInitialFaction_FirstTime_InsertsAndGrants(t *testing.T) {
	svc, playerRepo, factionRepo := newFactionTestEnv(t, "p1")

	err := svc.SelectInitialFaction(context.Background(), "p1", "SHE")
	require.NoError(t, err)

	factions, err := factionRepo.GetPlayerFactions(context.Background(), "p1")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"SHE"}, factions)

	p, err := playerRepo.FindByID(context.Background(), "p1")
	require.NoError(t, err)
	require.NotNil(t, p.SelectedFaction)
	assert.Equal(t, "SHE", *p.SelectedFaction)
}

func TestSelectInitialFaction_Repeat_NoopReturnsAlreadySelected(t *testing.T) {
	svc, _, _ := newFactionTestEnv(t, "p1")

	require.NoError(t, svc.SelectInitialFaction(context.Background(), "p1", "Tenki"))

	err := svc.SelectInitialFaction(context.Background(), "p1", "Tenki")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFactionAlreadySelected))
}

func TestSelectInitialFaction_InvalidFaction(t *testing.T) {
	svc, _, factionRepo := newFactionTestEnv(t, "p1")

	cases := []string{"Neutral", "bogus", ""}
	for _, f := range cases {
		err := svc.SelectInitialFaction(context.Background(), "p1", f)
		require.Errorf(t, err, "faction %q should be rejected", f)
		assert.Truef(t, errors.Is(err, ErrInvalidFaction),
			"faction %q should map to ErrInvalidFaction, got %v", f, err)
	}

	factions, err := factionRepo.GetPlayerFactions(context.Background(), "p1")
	require.NoError(t, err)
	assert.Empty(t, factions)
}


func TestSelectInitialFaction_EmptyPlayerID(t *testing.T) {
	svc, _, _ := newFactionTestEnv(t, "p1")

	err := svc.SelectInitialFaction(context.Background(), "", "SHE")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidFaction))
}
