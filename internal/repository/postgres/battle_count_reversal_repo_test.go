//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

// testGameID1 は battle の実発行形式 ("g-" + GUID 先頭 24 桁) を模した、UUID として
// 解釈できないダミー値であるため、game_id 列が TEXT で受理できることの確認を兼ねる。
const (
	testGameID1 = "g-aaaaaaaaaaaaaaaaaaaaaaaa"
)

func TestMarkReverted(t *testing.T) {
	repo := postgres.NewBattleCountReversalRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("battle_count_reversals への INSERT", func(t *testing.T) {
		// 冪等性ガード: 初回は created=true、重複 game_id は ON CONFLICT DO NOTHING で
		// created=false を返し、同一対戦への二重返却を検出する。
		noPreInsert := func(*testing.T) {}
		preInsertSameGame := func(t *testing.T) {
			_, err := sharedPg.Pool.Exec(ctx,
				`INSERT INTO account.battle_count_reversals (game_id) VALUES ($1)`,
				testGameID1)
			require.NoError(t, err)
		}

		tests := []struct {
			name        string
			preInsert   func(*testing.T)
			wantCreated bool
		}{
			{
				name:        "初回挿入のとき、created=true になる",
				preInsert:   noPreInsert,
				wantCreated: true,
			},
			{
				name:        "既存 game_id のとき、created=false になる (冪等ガード)",
				preInsert:   preInsertSameGame,
				wantCreated: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				tt.preInsert(t)

				created, err := repo.MarkReverted(ctx, testGameID1)
				require.NoError(t, err)
				assert.Equal(t, tt.wantCreated, created)
			})
		}
	})
}
