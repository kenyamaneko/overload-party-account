package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

func TestFactionRepository_AddAndGet(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name     string
		seed     []struct{ faction, source string }
		adds     []struct{ faction, source string }
		wantList []string
	}{
		{
			name:     "シードなし → 追加 1 件",
			adds:     []struct{ faction, source string }{{"SHE", "initial_selection"}},
			wantList: []string{"SHE"},
		},
		{
			name: "複数ファクション所持",
			adds: []struct{ faction, source string }{
				{"SHE", "initial_selection"},
				{"Tenki", "shop_purchase"},
			},
			wantList: []string{"SHE", "Tenki"},
		},
		{
			name: "重複 Add は冪等 (ON CONFLICT DO NOTHING)",
			adds: []struct{ faction, source string }{
				{"SHE", "initial_selection"},
				{"SHE", "shop_purchase"},
			},
			wantList: []string{"SHE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			for _, s := range tt.seed {
				seedPlayerFaction(t, testPlayerID1, s.faction, s.source)
			}
			for _, a := range tt.adds {
				require.NoError(t, repo.AddPlayerFaction(ctx, testPlayerID1, a.faction, a.source))
			}

			got, err := repo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantList, got)
		})
	}
}

func TestFactionRepository_InsertInitial(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name        string
		preExisting bool
		wantCreated bool
	}{
		{"初回は created=true", false, true},
		{"既存なら created=false (冪等)", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			if tt.preExisting {
				seedPlayerFaction(t, testPlayerID1, "SHE", "initial_selection")
			}

			created, err := repo.InsertInitial(ctx, testPlayerID1, "SHE", "initial_selection")
			require.NoError(t, err)
			assert.Equal(t, tt.wantCreated, created)
		})
	}
}

func TestFactionRepository_GetPlayerFactions_Empty(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	got, err := repo.GetPlayerFactions(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.Empty(t, got)
}
