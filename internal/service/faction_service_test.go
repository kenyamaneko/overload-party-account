package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFactionTestService は実 DB に playerID をシードし、実 repository で
// FactionService を組む。shop 流の「service テストも sharedPg で組む」方針。
func newFactionTestService(t *testing.T, playerID string) *FactionService {
	t.Helper()
	sharedPg.Truncate(t)
	seedPlayer(t, playerID, "uid-"+playerID, "tester", false)

	playerRepo, factionRepo, _, tx := newRealRepos()
	return NewFactionService(playerRepo, factionRepo, tx)
}

func TestFactionService_SelectInitialFaction_Success(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		faction string
	}{
		{
			name:    "SHE を初回選択",
			faction: "SHE",
		},
		{
			name:    "Tenki を初回選択",
			faction: "Tenki",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newFactionTestService(t, testPlayerID1)

			require.NoError(t, svc.SelectInitialFaction(ctx, testPlayerID1, tt.faction))

			playerRepo, factionRepo, _, _ := newRealRepos()

			factions, err := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.ElementsMatch(t, []string{tt.faction}, factions)

			p, err := playerRepo.FindByID(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, p.SelectedFaction)
			assert.Equal(t, tt.faction, *p.SelectedFaction)
		})
	}
}

func TestFactionService_SelectInitialFaction_ShopPrecededSucceeds(t *testing.T) {
	ctx := context.Background()

	// 初回選択済みか否かの SSoT は players.selected_faction。ショップで先に所持していても
	// selected_faction が NULL である限り初回選択は成立する。
	tests := []struct {
		name          string
		shopFaction   string
		initialChoice string
		wantOwned     []string
	}{
		{
			name:          "ショップで買ったのと同じファクションを初回選択",
			shopFaction:   "SHE",
			initialChoice: "SHE",
			wantOwned:     []string{"SHE"},
		},
		{
			name:          "ショップで買ったのと別のファクションを初回選択",
			shopFaction:   "SHE",
			initialChoice: "Tenki",
			wantOwned:     []string{"SHE", "Tenki"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newFactionTestService(t, testPlayerID1)
			playerRepo, factionRepo, _, _ := newRealRepos()

			require.NoError(t, factionRepo.AddPlayerFaction(ctx, testPlayerID1, tt.shopFaction, "shop_purchase"))
			require.NoError(t, svc.SelectInitialFaction(ctx, testPlayerID1, tt.initialChoice))

			factions, err := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantOwned, factions)

			p, err := playerRepo.FindByID(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, p.SelectedFaction)
			assert.Equal(t, tt.initialChoice, *p.SelectedFaction)
		})
	}
}

func TestFactionService_SelectInitialFaction_AlreadySelected_ReturnsError(t *testing.T) {
	ctx := context.Background()
	svc := newFactionTestService(t, testPlayerID1)

	require.NoError(t, svc.SelectInitialFaction(ctx, testPlayerID1, "Tenki"))

	err := svc.SelectInitialFaction(ctx, testPlayerID1, "Tenki")
	require.ErrorIs(t, err, ErrFactionAlreadySelected)

	// 二度目は副作用を起こしていない(faction 一覧・selected_faction ともに初回のまま)。
	playerRepo, factionRepo, _, _ := newRealRepos()

	factions, err := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Tenki"}, factions)

	p, err := playerRepo.FindByID(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, p.SelectedFaction)
	assert.Equal(t, "Tenki", *p.SelectedFaction)
}

func TestFactionService_SelectInitialFaction_InvalidFaction_ReturnsError(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		faction string
	}{
		{
			name:    "Neutral ファクションは選択不可",
			faction: "Neutral",
		},
		{
			name:    "未知ファクション",
			faction: "bogus",
		},
		{
			name:    "空ファクション",
			faction: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newFactionTestService(t, testPlayerID1)

			err := svc.SelectInitialFaction(ctx, testPlayerID1, tt.faction)
			require.ErrorIs(t, err, ErrInvalidFaction)

			// 不正入力は永続化しない。
			playerRepo, factionRepo, _, _ := newRealRepos()

			factions, err := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Empty(t, factions)

			p, err := playerRepo.FindByID(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Nil(t, p.SelectedFaction)
		})
	}
}

func TestFactionService_SelectInitialFaction_EmptyPlayerID_ReturnsError(t *testing.T) {
	ctx := context.Background()
	// 空 playerID は入力検証で弾かれるため、player をシードしなくても良い。
	sharedPg.Truncate(t)
	playerRepo, factionRepo, _, tx := newRealRepos()
	svc := NewFactionService(playerRepo, factionRepo, tx)

	err := svc.SelectInitialFaction(ctx, "", "SHE")
	require.ErrorIs(t, err, ErrInvalidFaction)
}
