//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

func TestFactionRepository_AddPlayerFaction(t *testing.T) {
	t.Run("[FactionRepository]所持陣営の永続化", func(t *testing.T) {
		t.Run("ファクションの追加", func(t *testing.T) {
			t.Run("未所持のファクションを追加すると、is_initial=falseの所持ファクションとして記録する", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewFactionRepository(sharedPg.Pool)

				err := repo.AddPlayerFaction(context.Background(), player.PlayerID, gamedesign.FactionSHE)

				require.NoError(t, err)
				factions, err := repo.GetPlayerFactions(context.Background(), player.PlayerID)
				require.NoError(t, err)
				assert.Contains(t, factions, gamedesign.FactionSHE)
				initial, err := repo.GetInitialFaction(context.Background(), player.PlayerID)
				require.NoError(t, err)
				assert.Nil(t, initial)
			})

			t.Run("既に所持しているファクションを重ねて追加しても、エラーにはならず状態は変化しない(冪等)", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewFactionRepository(sharedPg.Pool)
				require.NoError(t, repo.AddPlayerFaction(context.Background(), player.PlayerID, gamedesign.FactionSHE))

				err := repo.AddPlayerFaction(context.Background(), player.PlayerID, gamedesign.FactionSHE)

				require.NoError(t, err)
				factions, err := repo.GetPlayerFactions(context.Background(), player.PlayerID)
				require.NoError(t, err)
				assert.ElementsMatch(t, []string{gamedesign.FactionSHE}, factions)
			})
		})
	})
}

func TestFactionRepository_SetInitialFaction(t *testing.T) {
	t.Run("[FactionRepository]所持陣営の永続化", func(t *testing.T) {
		t.Run("SetInitialFaction", func(t *testing.T) {
			t.Run("対象プレイヤーがそのファクションを未所持のとき、is_initial=trueの行を新規作成する", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewFactionRepository(sharedPg.Pool)

				err := repo.SetInitialFaction(context.Background(), player.PlayerID, gamedesign.FactionSHE)

				require.NoError(t, err)
				initial, err := repo.GetInitialFaction(context.Background(), player.PlayerID)
				require.NoError(t, err)
				require.NotNil(t, initial)
				assert.Equal(t, gamedesign.FactionSHE, *initial)
			})

			t.Run("対象プレイヤーが既にそのファクションを(is_initial=falseで)所持しているとき、is_initialをtrueに更新する", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewFactionRepository(sharedPg.Pool)
				require.NoError(t, repo.AddPlayerFaction(context.Background(), player.PlayerID, gamedesign.FactionSHE))

				err := repo.SetInitialFaction(context.Background(), player.PlayerID, gamedesign.FactionSHE)

				require.NoError(t, err)
				initial, err := repo.GetInitialFaction(context.Background(), player.PlayerID)
				require.NoError(t, err)
				require.NotNil(t, initial)
				assert.Equal(t, gamedesign.FactionSHE, *initial)
			})
		})
	})
}

func TestFactionRepository_GetInitialFaction(t *testing.T) {
	t.Run("[FactionRepository]所持陣営の永続化", func(t *testing.T) {
		t.Run("GetInitialFaction", func(t *testing.T) {
			t.Run("initialファクションが未選択のとき、エラーにはせずnilを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewFactionRepository(sharedPg.Pool)

				initial, err := repo.GetInitialFaction(context.Background(), player.PlayerID)

				require.NoError(t, err)
				assert.Nil(t, initial)
			})

			t.Run("initialファクションが選択済みのとき、その値を返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewFactionRepository(sharedPg.Pool)
				require.NoError(t, repo.SetInitialFaction(context.Background(), player.PlayerID, gamedesign.FactionTenki))

				initial, err := repo.GetInitialFaction(context.Background(), player.PlayerID)

				require.NoError(t, err)
				require.NotNil(t, initial)
				assert.Equal(t, gamedesign.FactionTenki, *initial)
			})
		})
	})
}

func TestFactionRepository_GetPlayerFactions(t *testing.T) {
	t.Run("[FactionRepository]所持陣営の永続化", func(t *testing.T) {
		t.Run("所持ファクション一覧の取得", func(t *testing.T) {
			t.Run("所持ファクションが無いとき、要素数0の一覧を返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewFactionRepository(sharedPg.Pool)

				factions, err := repo.GetPlayerFactions(context.Background(), player.PlayerID)

				require.NoError(t, err)
				assert.Empty(t, factions)
			})

			t.Run("所持ファクションが複数あるとき、その一覧を返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewFactionRepository(sharedPg.Pool)
				require.NoError(t, repo.AddPlayerFaction(context.Background(), player.PlayerID, gamedesign.FactionSHE))
				require.NoError(t, repo.AddPlayerFaction(context.Background(), player.PlayerID, gamedesign.FactionSugar))

				factions, err := repo.GetPlayerFactions(context.Background(), player.PlayerID)

				require.NoError(t, err)
				assert.ElementsMatch(t, []string{gamedesign.FactionSHE, gamedesign.FactionSugar}, factions)
			})
		})
	})
}
