//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

func TestPlayerViewFindByID(t *testing.T) {
	repo := postgres.NewPlayerViewRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("player_idによるプレイヤービュー取得", func(t *testing.T) {
		tests := []struct {
			name               string
			seedFaction        func(t *testing.T)
			wantInitialFaction *string
		}{
			{
				name: "初期陣営確定済みのプレイヤーを取得すると、初期陣営SHEが入る",
				seedFaction: func(t *testing.T) {
					seedPlayerFaction(t, testPlayerID1, "SHE", true)
				},
				wantInitialFaction: ptr("SHE"),
			},
			{
				name:               "陣営未選択のプレイヤーを取得すると、初期陣営はnilになる",
				seedFaction:        func(t *testing.T) {},
				wantInitialFaction: nil,
			},
			{
				name: "初期でない陣営だけ所持しているとき、初期陣営はnilになる",
				seedFaction: func(t *testing.T) {
					seedPlayerFaction(t, testPlayerID1, "SHE", false)
				},
				wantInitialFaction: nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
				tt.seedFaction(t)

				got, err := repo.FindByID(ctx, testPlayerID1)
				require.NoError(t, err)
				assert.Equal(t, tt.wantInitialFaction, got.InitialFaction)
			})
		}
	})
}

func TestPlayerViewFindByFirebaseUID(t *testing.T) {
	repo := postgres.NewPlayerViewRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("firebase_uidによるプレイヤービュー取得", func(t *testing.T) {
		t.Run("初期陣営確定済みのプレイヤーを取得すると、初期陣営SHEが入る", func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedPlayerFaction(t, testPlayerID1, "SHE", true)

			got, err := repo.FindByFirebaseUID(ctx, "uid-1")
			require.NoError(t, err)
			require.NotNil(t, got.InitialFaction)
			assert.Equal(t, "SHE", *got.InitialFaction)
		})
	})
}
