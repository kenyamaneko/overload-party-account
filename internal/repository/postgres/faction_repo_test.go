//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

type factionAdd struct {
	playerID string
	faction  string
	source   string
}

// AddPlayerFaction の契約:
//   - ON CONFLICT (player_id, faction) DO NOTHING による一意性保証と冪等性
//   - スコープは player_id 単位 (別プレイヤーは同一ファクションを独立して保持できる)
//
// 状態確認には GetPlayerFactions を使う（独立テストで Get 側の契約は別途検証）。
func TestFactionRepository_AddPlayerFaction(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name      string
		adds      []factionAdd
		wantByPID map[string][]string
	}{
		{
			name: "新規ファクションを追加する",
			adds: []factionAdd{
				{
					playerID: testPlayerID1,
					faction:  "SHE",
					source:   "initial_selection",
				},
			},
			wantByPID: map[string][]string{
				testPlayerID1: {"SHE"},
			},
		},
		{
			name: "異なるファクションを重ねて追加できる",
			adds: []factionAdd{
				{
					playerID: testPlayerID1,
					faction:  "SHE",
					source:   "initial_selection",
				},
				{
					playerID: testPlayerID1,
					faction:  "Tenki",
					source:   "shop_purchase",
				},
			},
			wantByPID: map[string][]string{
				testPlayerID1: {"SHE", "Tenki"},
			},
		},
		{
			name: "同一プレイヤー×同一ファクションの重複追加は冪等で行が増えない",
			adds: []factionAdd{
				{
					playerID: testPlayerID1,
					faction:  "SHE",
					source:   "initial_selection",
				},
				{
					playerID: testPlayerID1,
					faction:  "SHE",
					source:   "shop_purchase",
				},
			},
			wantByPID: map[string][]string{
				testPlayerID1: {"SHE"},
			},
		},
		{
			name: "別プレイヤーは同一ファクションを独立して保持できる",
			adds: []factionAdd{
				{
					playerID: testPlayerID1,
					faction:  "SHE",
					source:   "initial_selection",
				},
				{
					playerID: testPlayerID2,
					faction:  "SHE",
					source:   "initial_selection",
				},
			},
			wantByPID: map[string][]string{
				testPlayerID1: {"SHE"},
				testPlayerID2: {"SHE"},
			},
		},
		{
			name: "プレイヤーごとに所持ファクションが混在せず分離される",
			adds: []factionAdd{
				{
					playerID: testPlayerID1,
					faction:  "SHE",
					source:   "initial_selection",
				},
				{
					playerID: testPlayerID1,
					faction:  "Tenki",
					source:   "shop_purchase",
				},
				{
					playerID: testPlayerID2,
					faction:  "SHE",
					source:   "initial_selection",
				},
			},
			wantByPID: map[string][]string{
				testPlayerID1: {"SHE", "Tenki"},
				testPlayerID2: {"SHE"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedPlayer(t, testPlayerID2, "uid-2", "Bob", false)

			for _, a := range tt.adds {
				require.NoError(t, repo.AddPlayerFaction(ctx, a.playerID, a.faction, a.source))
			}

			for pid, want := range tt.wantByPID {
				got, err := repo.GetPlayerFactions(ctx, pid)
				require.NoError(t, err)
				assert.ElementsMatch(t, want, got)
			}
		})
	}
}

// GetPlayerFactions の契約: プレイヤーの所持ファクションを過不足なく返す。
// seed には AddPlayerFaction を使う（Add 側の契約は別テストで検証）。
func TestFactionRepository_GetPlayerFactions(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name     string
		seed     []factionAdd
		wantList []string
	}{
		{
			name:     "所持なしは空配列 (新規プレイヤーの正常系)",
			seed:     nil,
			wantList: []string{},
		},
		{
			name: "単一ファクションを所持",
			seed: []factionAdd{
				{
					playerID: testPlayerID1,
					faction:  "SHE",
					source:   "initial_selection",
				},
			},
			wantList: []string{"SHE"},
		},
		{
			name: "複数ファクションを所持",
			seed: []factionAdd{
				{
					playerID: testPlayerID1,
					faction:  "SHE",
					source:   "initial_selection",
				},
				{
					playerID: testPlayerID1,
					faction:  "Tenki",
					source:   "shop_purchase",
				},
			},
			wantList: []string{"SHE", "Tenki"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			for _, s := range tt.seed {
				require.NoError(t, repo.AddPlayerFaction(ctx, s.playerID, s.faction, s.source))
			}

			got, err := repo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantList, got)
		})
	}
}
