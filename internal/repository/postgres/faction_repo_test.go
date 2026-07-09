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

	t.Run("player_factions への追加", func(t *testing.T) {
		tests := []struct {
			name      string
			adds      []string // 追加対象 faction 名 (testPlayerID1)
			extraPID2 []string // testPlayerID2 への追加 faction 名
			wantByPID map[string][]string
		}{
			{
				name: "新規ファクションを追加するとき、所持一覧に現れる",
				adds: []string{"SHE"},
				wantByPID: map[string][]string{
					testPlayerID1: {"SHE"},
				},
			},
			{
				name: "異なるファクションを重ねて追加するとき、両方を所持する",
				adds: []string{"SHE", "Tenki"},
				wantByPID: map[string][]string{
					testPlayerID1: {"SHE", "Tenki"},
				},
			},
			{
				name: "同一プレイヤーに同一ファクションを重複追加しても、行が増えない",
				adds: []string{"SHE", "SHE"},
				wantByPID: map[string][]string{
					testPlayerID1: {"SHE"},
				},
			},
			{
				name:      "別プレイヤーが同一ファクションを持つとき、独立して保持される",
				adds:      []string{"SHE"},
				extraPID2: []string{"SHE"},
				wantByPID: map[string][]string{
					testPlayerID1: {"SHE"},
					testPlayerID2: {"SHE"},
				},
			},
			{
				name:      "複数プレイヤーが別々に所持するとき、混在せず分離される",
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
	})
}

func TestSetInitialFaction(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("初期ファクションの設定", func(t *testing.T) {
		tests := []struct {
			name        string
			seed        []factionAdd
			wantErr     bool
			wantInitial *string
		}{
			{
				name:        "対象行が無いとき、新規 INSERT され initial になる",
				seed:        nil,
				wantErr:     false,
				wantInitial: ptr("SHE"),
			},
			{
				name: "別プレイヤーが同 faction を所持していても、initial 選択に影響しない",
				seed: []factionAdd{
					{playerID: testPlayerID2, faction: "SHE", isInitial: false},
				},
				wantErr:     false,
				wantInitial: ptr("SHE"),
			},
			{
				name: "(player_id, faction) が PK 重複のとき、エラーになる",
				seed: []factionAdd{
					{playerID: testPlayerID1, faction: "SHE", isInitial: false},
				},
				wantErr:     true,
				wantInitial: nil,
			},
			{
				name: "別 faction が既に is_initial=TRUE のとき、partial unique index 違反でエラーになる",
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
	})
}

func TestGetPlayerFactions(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("所持ファクション一覧の取得", func(t *testing.T) {
		tests := []struct {
			name     string
			seed     []factionAdd
			wantList []string
		}{
			{
				name:     "所持なしのとき、空配列を返す",
				seed:     nil,
				wantList: []string{},
			},
			{
				name: "単一ファクションを所持するとき、その 1 件を返す",
				seed: []factionAdd{
					{playerID: testPlayerID1, faction: "SHE", isInitial: true},
				},
				wantList: []string{"SHE"},
			},
			{
				name: "initial と非 initial が混在して所持するとき、全件を返す",
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
	})
}

func TestGetInitialFaction(t *testing.T) {
	repo := postgres.NewFactionRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("初期ファクションの取得", func(t *testing.T) {
		tests := []struct {
			name string
			seed []factionAdd
			want *string
		}{
			{
				name: "未選択のとき、nil を返す",
				seed: nil,
				want: nil,
			},
			{
				name: "is_initial=FALSE の行のみのとき、nil を返す",
				seed: []factionAdd{
					{playerID: testPlayerID1, faction: "SHE", isInitial: false},
				},
				want: nil,
			},
			{
				name: "is_initial=TRUE の行があるとき、その faction 名を返す",
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
	})
}
