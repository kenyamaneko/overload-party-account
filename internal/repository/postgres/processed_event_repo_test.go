//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

func TestProcessedEventRepository_Insert(t *testing.T) {
	t.Run("[ProcessedEventRepository]処理済みイベントの記録", func(t *testing.T) {
		t.Run("冪等ガード", func(t *testing.T) {
			t.Run("新規のevent_idを指定したとき、記録を新規作成しtrueを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewProcessedEventRepository(sharedPg.Pool)

				inserted, err := repo.Insert(context.Background(), uuid.NewString(), "faction_acquired")

				require.NoError(t, err)
				assert.True(t, inserted)
			})

			t.Run("既に同じevent_idの記録が存在するとき、記録を追加せずfalseを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewProcessedEventRepository(sharedPg.Pool)
				eventID := uuid.NewString()
				_, err := repo.Insert(context.Background(), eventID, "faction_acquired")
				require.NoError(t, err)

				inserted, err := repo.Insert(context.Background(), eventID, "faction_acquired")

				require.NoError(t, err)
				assert.False(t, inserted)
			})
		})
	})
}
