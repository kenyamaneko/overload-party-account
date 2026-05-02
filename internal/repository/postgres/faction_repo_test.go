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
	playerID  string
	faction   string
	isInitial bool
}

func TestAddPlayerFaction(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name      string
		adds      []string // 追加対象 faction 名 (testPlayerID1)
		extraPID2 []string // testPlayerID2 への追加 faction 名
		wantByPID map[string][]string
	}{
		{
			name: "新規ファクションを追加する",
			adds: []string{"SHE"},
			wantByPID: map[string][]string{
				testPlayerID1: {"SHE"},
			},
		},
		{
			name: "異なるファクションを重ねて追加できる",
			adds: []string{"SHE", "Tenki"},
			wantByPID: map[string][]string{
				testPlayerID1: {"SHE", "Tenki"},
			},
		},
		{
			name: "同一プレイヤー×同一ファクションの重複追加は冪等で行が増えない",
			adds: []string{"SHE", "SHE"},
			wantByPID: map[string][]string{
				testPlayerID1: {"SHE"},
			},
		},
		{
			name:      "別プレイヤーは同一ファクションを独立して保持できる",
			adds:      []string{"SHE"},
			extraPID2: []string{"SHE"},
			wantByPID: map[string][]string{
				testPlayerID1: {"SHE"},
				testPlayerID2: {"SHE"},
			},
		},
		{
			name:      "プレイヤーごとに所持ファクションが混在せず分離される",
			adds:      []string{"SHE", "Tenki"},
			extraPID2: []string{"SHE"},
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

			for _, f := range tt.adds {
				require.NoError(t, repo.AddPlayerFaction(ctx, testPlayerID1, f))
			}
			for _, f := range tt.extraPID2 {
				require.NoError(t, repo.AddPlayerFaction(ctx, testPlayerID2, f))
			}

			for pid, want := range tt.wantByPID {
				got, err := repo.GetPlayerFactions(ctx, pid)
				require.NoError(t, err)
				assert.ElementsMatch(t, want, got)
			}
		})
	}
}

func TestSetInitialFaction(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name        string
		seed        []factionAdd
		wantErr     bool
		wantInitial *string
	}{
		{
			name:        "行が無ければ新規 INSERT",
			seed:        nil,
			wantErr:     false,
			wantInitial: ptr("SHE"),
		},
		{
			name: "別プレイヤーが同 faction を所持していても影響しない",
			seed: []factionAdd{
				{playerID: testPlayerID2, faction: "SHE", isInitial: false},
			},
			wantErr:     false,
			wantInitial: ptr("SHE"),
		},
		{
			name: "(player_id, faction) PK 重複でエラー",
			seed: []factionAdd{
				{playerID: testPlayerID1, faction: "SHE", isInitial: false},
			},
			wantErr:     true,
			wantInitial: nil,
		},
		{
			name: "別 faction が既に is_initial=TRUE で partial unique index 違反",
			seed: []factionAdd{
				{playerID: testPlayerID1, faction: "Tenki", isInitial: true},
			},
			wantErr:     true,
			wantInitial: ptr("Tenki"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedPlayer(t, testPlayerID2, "uid-2", "Bob", false)
			for _, s := range tt.seed {
				seedPlayerFaction(t, s.playerID, s.faction, s.isInitial)
			}

			err := repo.SetInitialFaction(ctx, testPlayerID1, "SHE")
			require.Equal(t, tt.wantErr, err != nil)

			got, err := repo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantInitial, got)
		})
	}
}

func TestGetPlayerFactions(t *testing.T) {
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
				{playerID: testPlayerID1, faction: "SHE", isInitial: true},
			},
			wantList: []string{"SHE"},
		},
		{
			name: "複数ファクションを所持 (initial と非 initial が混在)",
			seed: []factionAdd{
				{playerID: testPlayerID1, faction: "SHE", isInitial: true},
				{playerID: testPlayerID1, faction: "Tenki", isInitial: false},
			},
			wantList: []string{"SHE", "Tenki"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			for _, s := range tt.seed {
				seedPlayerFaction(t, s.playerID, s.faction, s.isInitial)
			}

			got, err := repo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantList, got)
		})
	}
}

func TestGetInitialFaction(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name string
		seed []factionAdd
		want *string
	}{
		{
			name: "未選択なら nil",
			seed: nil,
			want: nil,
		},
		{
			name: "is_initial=FALSE の行のみなら nil",
			seed: []factionAdd{
				{playerID: testPlayerID1, faction: "SHE", isInitial: false},
			},
			want: nil,
		},
		{
			name: "is_initial=TRUE の行があれば faction 名を返す",
			seed: []factionAdd{
				{playerID: testPlayerID1, faction: "SHE", isInitial: false},
				{playerID: testPlayerID1, faction: "Tenki", isInitial: true},
			},
			want: ptr("Tenki"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			for _, s := range tt.seed {
				seedPlayerFaction(t, s.playerID, s.faction, s.isInitial)
			}

			got, err := repo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
