//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// newFactionTestService は実 DB に playerID をシードし、実 repository で
// FactionInteractor を組む。shop 流の「usecase テストも sharedPg で組む」方針。
func newFactionTestService(t *testing.T, playerID string) *FactionInteractor {
	t.Helper()
	sharedPg.Truncate(t)
	seedPlayer(t, playerID, "uid-"+playerID, "tester", false)

	playerRepo, _, factionRepo, _, tx := newRealRepos()
	return NewFactionInteractor(playerRepo, factionRepo, tx)
}

func TestFactionInteractor_SelectInitialFaction_Success(t *testing.T) {
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

			_, _, factionRepo, _, _ := newRealRepos()

			factions, err := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.ElementsMatch(t, []string{tt.faction}, factions)

			initial, err := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, initial)
			assert.Equal(t, tt.faction, *initial)
		})
	}
}

// ショップで所持していない faction を initial 選択すると成立する。
// ショップで所持している faction と同一のものを initial 選択すると PK 重複でエラー。
func TestFactionInteractor_SelectInitialFaction_WithShopOwnedFaction(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		shopFaction   string
		initialChoice string
		wantOwned     []string
	}{
		{
			name:          "別 faction を initial 選択",
			shopFaction:   "SHE",
			initialChoice: "Tenki",
			wantOwned:     []string{"SHE", "Tenki"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newFactionTestService(t, testPlayerID1)
			_, _, factionRepo, _, _ := newRealRepos()

			require.NoError(t, factionRepo.AddPlayerFaction(ctx, testPlayerID1, tt.shopFaction))
			require.NoError(t, svc.SelectInitialFaction(ctx, testPlayerID1, tt.initialChoice))

			factions, err := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantOwned, factions)

			initial, err := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, initial)
			assert.Equal(t, tt.initialChoice, *initial)
		})
	}
}

func TestFactionInteractor_SelectInitialFaction_SameAsShopOwned_ReturnsError(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		faction string
	}{
		{name: "ショップで所持中の SHE を initial 選択", faction: "SHE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newFactionTestService(t, testPlayerID1)
			_, _, factionRepo, _, _ := newRealRepos()

			require.NoError(t, factionRepo.AddPlayerFaction(ctx, testPlayerID1, tt.faction))

			err := svc.SelectInitialFaction(ctx, testPlayerID1, tt.faction)
			assert.Error(t, err)
		})
	}
}

func TestFactionInteractor_SelectInitialFaction_AlreadySelected_ReturnsError(t *testing.T) {
	ctx := context.Background()
	svc := newFactionTestService(t, testPlayerID1)

	require.NoError(t, svc.SelectInitialFaction(ctx, testPlayerID1, "Tenki"))

	err := svc.SelectInitialFaction(ctx, testPlayerID1, "Tenki")
	require.ErrorIs(t, err, ErrFactionAlreadySelected)

	// 二度目は副作用を起こしていない (faction 一覧・initial faction ともに初回のまま)。
	_, _, factionRepo, _, _ := newRealRepos()

	factions, err := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Tenki"}, factions)

	initial, err := factionRepo.GetInitialFaction(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, initial)
	assert.Equal(t, "Tenki", *initial)
}

func TestFactionInteractor_SelectInitialFaction_InvalidFaction_ReturnsError(t *testing.T) {
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
			_, _, factionRepo, _, _ := newRealRepos()

			factions, err := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Empty(t, factions)

			initial, err := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Nil(t, initial)
		})
	}
}

func TestFactionInteractor_SelectInitialFaction_EmptyPlayerID_ReturnsError(t *testing.T) {
	ctx := context.Background()
	// 空 playerID は入力検証で弾かれるため、player をシードしなくても良い。
	sharedPg.Truncate(t)
	playerRepo, _, factionRepo, _, tx := newRealRepos()
	svc := NewFactionInteractor(playerRepo, factionRepo, tx)

	err := svc.SelectInitialFaction(ctx, "", "SHE")
	require.ErrorIs(t, err, ErrInvalidFaction)
}

// プレイヤーが存在しない場合は ErrNotFound を返す。
// repo 層は「行が更新されたか」しか返さないため、usecase が Exists で識別する責務を持つ。
func TestFactionInteractor_SelectInitialFaction_PlayerNotFound_ReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	playerRepo, _, factionRepo, _, tx := newRealRepos()
	svc := NewFactionInteractor(playerRepo, factionRepo, tx)

	err := svc.SelectInitialFaction(ctx, "99999999-9999-9999-9999-999999999999", "SHE")
	require.ErrorIs(t, err, port.ErrNotFound)
}
