package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

type factionEntry struct {
	faction string
	source  string
}

func TestFactionRepository_AddAndGet(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name     string
		adds     []factionEntry
		wantList []string
	}{
		{
			name:     "追加なしは空配列",
			adds:     nil,
			wantList: []string{},
		},
		{
			name: "追加 1 件",
			adds: []factionEntry{
				{faction: "SHE", source: "initial_selection"},
			},
			wantList: []string{"SHE"},
		},
		{
			name: "複数ファクション所持",
			adds: []factionEntry{
				{faction: "SHE", source: "initial_selection"},
				{faction: "Tenki", source: "shop_purchase"},
			},
			wantList: []string{"SHE", "Tenki"},
		},
		{
			name: "重複 Add は冪等 (ON CONFLICT DO NOTHING)",
			adds: []factionEntry{
				{faction: "SHE", source: "initial_selection"},
				{faction: "SHE", source: "shop_purchase"},
			},
			wantList: []string{"SHE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

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
		{
			name:        "初回は created=true",
			preExisting: false,
			wantCreated: true,
		},
		{
			name:        "既存なら created=false (冪等)",
			preExisting: true,
			wantCreated: false,
		},
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
