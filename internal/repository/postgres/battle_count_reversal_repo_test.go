//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

func TestBattleCountReversalRepository_MarkReverted(t *testing.T) {
	t.Run("[BattleCountReversalRepository]バトル回数取り消しの記録", func(t *testing.T) {
		t.Run("冪等ガード", func(t *testing.T) {
			t.Run("新規のgame_idを指定したとき、記録を新規作成しtrueを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewBattleCountReversalRepository(sharedPg.Pool)

				marked, err := repo.MarkReverted(context.Background(), "game-1")

				require.NoError(t, err)
				assert.True(t, marked)
			})

			t.Run("既に同じgame_idの記録が存在するとき、記録を追加せずfalseを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewBattleCountReversalRepository(sharedPg.Pool)
				_, err := repo.MarkReverted(context.Background(), "game-1")
				require.NoError(t, err)

				marked, err := repo.MarkReverted(context.Background(), "game-1")

				require.NoError(t, err)
				assert.False(t, marked)
			})
		})
	})
}
