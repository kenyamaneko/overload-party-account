//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

func TestPlayerViewRepository_ReferenceMethodsNotFound(t *testing.T) {
	t.Run("PlayerViewRepository", func(t *testing.T) {
		t.Run("参照系メソッドに共通する仕様", func(t *testing.T) {
			t.Run("FindByIDは、存在しないplayer_idを指定したとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerViewRepository(sharedPg.Pool)

				_, err := repo.FindByID(context.Background(), uuid.NewString())

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("FindByFirebaseUIDは、存在しないfirebase_uidを指定したとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerViewRepository(sharedPg.Pool)

				_, err := repo.FindByFirebaseUID(context.Background(), "missing-firebase-uid")

				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		})
	})
}

func TestPlayerViewRepository_JoinedFields(t *testing.T) {
	t.Run("PlayerViewRepository", func(t *testing.T) {
		t.Run("FindByIDは、players/player_progression/player_factions(is_initial=true)を結合した結果を返す", func(t *testing.T) {
			sharedPg.Truncate(t)
			player := createTestPlayer(t)
			factionRepo := postgres.NewFactionRepository(sharedPg.Pool)
			require.NoError(t, factionRepo.SetInitialFaction(context.Background(), player.PlayerID, gamedesign.FactionSHE))
			viewRepo := postgres.NewPlayerViewRepository(sharedPg.Pool)

			view, err := viewRepo.FindByID(context.Background(), player.PlayerID)

			require.NoError(t, err)
			assert.Equal(t, player.PlayerID, view.Player.PlayerID)
			assert.Equal(t, int64(1), view.Level)
			assert.Equal(t, int64(0), view.Exp)
			require.NotNil(t, view.InitialFaction)
			assert.Equal(t, gamedesign.FactionSHE, *view.InitialFaction)
		})

		t.Run("初期ファクション未選択のプレイヤーについては、InitialFactionはnilとして返る", func(t *testing.T) {
			sharedPg.Truncate(t)
			player := createTestPlayer(t)
			viewRepo := postgres.NewPlayerViewRepository(sharedPg.Pool)

			view, err := viewRepo.FindByID(context.Background(), player.PlayerID)

			require.NoError(t, err)
			assert.Nil(t, view.InitialFaction)
		})

		t.Run("FindByFirebaseUIDは、players/player_progression/player_factions(is_initial=true)を結合した結果を返す", func(t *testing.T) {
			sharedPg.Truncate(t)
			player := createTestPlayer(t)
			viewRepo := postgres.NewPlayerViewRepository(sharedPg.Pool)

			view, err := viewRepo.FindByFirebaseUID(context.Background(), player.FirebaseUID)

			require.NoError(t, err)
			assert.Equal(t, player.PlayerID, view.Player.PlayerID)
		})
	})
}
