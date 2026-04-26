//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// newFactionTestService は実 DB に playerID をシードし、実 repository で
// FactionService を組む。shop 流の「service テストも sharedPg で組む」方針。
func newFactionTestService(t *testing.T, playerID string) *FactionService {
	t.Helper()
	sharedPg.Truncate(t)
	seedPlayer(t, playerID, "uid-"+playerID, "tester", false)

	playerRepo, factionRepo, _, tx := newRealRepos()
	eventRepo := newProcessedEventRepo()
	return NewFactionService(playerRepo, factionRepo, eventRepo, tx)
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

			require.NoError(t, factionRepo.AddPlayerFaction(ctx, testPlayerID1, tt.shopFaction, FactionSourceShopPurchase))
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
	eventRepo := newProcessedEventRepo()
	svc := NewFactionService(playerRepo, factionRepo, eventRepo, tx)

	err := svc.SelectInitialFaction(ctx, "", "SHE")
	require.ErrorIs(t, err, ErrInvalidFaction)
}

// プレイヤーが存在しない場合は ErrNotFound を返す。
// repo 層は「行が更新されたか」しか返さないため、service が Exists で識別する責務を持つ。
func TestFactionService_SelectInitialFaction_PlayerNotFound_ReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	playerRepo, factionRepo, _, tx := newRealRepos()
	eventRepo := newProcessedEventRepo()
	svc := NewFactionService(playerRepo, factionRepo, eventRepo, tx)

	err := svc.SelectInitialFaction(ctx, "99999999-9999-9999-9999-999999999999", "SHE")
	require.ErrorIs(t, err, port.ErrNotFound)
}

// ApplyOnboardingResult は表示名を扱わない契約: シナリオは入力時点で
// PUT /players/:id/name を呼んで account に確定済みのため、player-onboarded 経路では
// initial_faction の反映と processed_events の冪等ガードのみを担う。
// name が NULL のままでも faction の付与には支障がないことをここで固定する。
func TestFactionService_ApplyOnboardingResult_NameIndependent(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "", false) // name 未確定 (NULL) のプレイヤー

	playerRepo, factionRepo, _, tx := newRealRepos()
	eventRepo := newProcessedEventRepo()
	svc := NewFactionService(playerRepo, factionRepo, eventRepo, tx)

	processed, selected, err := svc.ApplyOnboardingResult(
		ctx,
		"44444444-4444-4444-4444-444444444444",
		"player.onboarded",
		testPlayerID1,
		"SHE",
	)
	require.NoError(t, err)
	assert.True(t, processed)
	assert.True(t, selected)

	// faction だけ反映され、name は触られず NULL のまま。
	p, ferr := playerRepo.FindByID(ctx, testPlayerID1)
	require.NoError(t, ferr)
	assert.Nil(t, p.Name, "ApplyOnboardingResult は name を書かない")
	require.NotNil(t, p.SelectedFaction)
	assert.Equal(t, "SHE", *p.SelectedFaction)

	factions, ferr := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
	require.NoError(t, ferr)
	assert.ElementsMatch(t, []string{"SHE"}, factions)
}
