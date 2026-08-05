//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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

	t.Run("player_factionsへの追加", func(t *testing.T) {
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
			name         string
			seed         []factionAdd
			setCalls     int
			wantInitial  *string
			wantFactions []string
		}{
			{
				name: "ショップで購入済みのファクションを初期選択すると、そのファクションが初期選択として記録される",
				seed: []factionAdd{
					{playerID: testPlayerID1, faction: "SHE", isInitial: false},
				},
				setCalls:     1,
				wantInitial:  ptr("SHE"),
				wantFactions: []string{"SHE"},
			},
			{
				name:         "未所持のファクションを初期選択すると、所持と初期選択の両方が記録される",
				seed:         nil,
				setCalls:     1,
				wantInitial:  ptr("SHE"),
				wantFactions: []string{"SHE"},
			},
			{
				name:         "同じ初期選択が二度届いても、記録が変わらない",
				seed:         nil,
				setCalls:     2,
				wantInitial:  ptr("SHE"),
				wantFactions: []string{"SHE"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
				for _, s := range tt.seed {
					seedPlayerFaction(t, s.playerID, s.faction, s.isInitial)
				}

				for i := 0; i < tt.setCalls; i++ {
					require.NoError(t, repo.SetInitialFaction(ctx, testPlayerID1, "SHE"))
				}

				gotInitial, err := repo.GetInitialFaction(ctx, testPlayerID1)
				require.NoError(t, err)
				assert.Equal(t, tt.wantInitial, gotInitial)

				gotFactions, err := repo.GetPlayerFactions(ctx, testPlayerID1)
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.wantFactions, gotFactions)
			})
		}

		t.Run("別プレイヤーが同じファクションを購入済みでも、初期選択は選んだプレイヤーだけに記録される", func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedPlayer(t, testPlayerID2, "uid-2", "Bob", false)
			seedPlayerFaction(t, testPlayerID2, "SHE", false)

			require.NoError(t, repo.SetInitialFaction(ctx, testPlayerID1, "SHE"))

			gotInitial1, err := repo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, ptr("SHE"), gotInitial1)

			gotInitial2, err := repo.GetInitialFaction(ctx, testPlayerID2)
			require.NoError(t, err)
			assert.Nil(t, gotInitial2)
		})

		t.Run("別のファクションが初期選択済みのとき、購入済みファクションの初期選択は失敗し、先の初期選択が残る", func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedPlayerFaction(t, testPlayerID1, "Tenki", true)
			seedPlayerFaction(t, testPlayerID1, "SHE", false)

			err := repo.SetInitialFaction(ctx, testPlayerID1, "SHE")

			var pgErr *pgconn.PgError
			require.ErrorAs(t, err, &pgErr)
			assert.Equal(t, "23505", pgErr.Code)
			assert.Equal(t, "idx_player_factions_initial", pgErr.ConstraintName)

			gotInitial, err := repo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, ptr("Tenki"), gotInitial)
		})
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
				name: "単一ファクションを所持するとき、その1件を返す",
				seed: []factionAdd{
					{playerID: testPlayerID1, faction: "SHE", isInitial: true},
				},
				wantList: []string{"SHE"},
			},
			{
				name: "initialと非initialが混在して所持するとき、全件を返す",
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
				name: "未選択のとき、nilを返す",
				seed: nil,
				want: nil,
			},
			{
				name: "is_initial=FALSEの行のみのとき、nilを返す",
				seed: []factionAdd{
					{playerID: testPlayerID1, faction: "SHE", isInitial: false},
				},
				want: nil,
			},
			{
				name: "is_initial=TRUEの行があるとき、そのfaction名を返す",
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
