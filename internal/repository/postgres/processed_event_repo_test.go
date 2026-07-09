//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

const (
	testEventID1 = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
)

func TestInsert_ProcessedEvent(t *testing.T) {
	repo := postgres.NewProcessedEventRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("processed_events への INSERT", func(t *testing.T) {
		// 冪等性ガード: 初回は created=true、重複 event_id は ON CONFLICT DO NOTHING で
		// created=false を返し、Pub/Sub の重複配信を検出する。
		noPreInsert := func(*testing.T) {}
		preInsertSameEvent := func(t *testing.T) {
			_, err := sharedPg.Pool.Exec(ctx,
				`INSERT INTO account.processed_events (event_id, event_type) VALUES ($1, $2)`,
				testEventID1, "faction_selected")
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
				name:        "既存 event_id のとき、created=false になる (冪等ガード)",
				preInsert:   preInsertSameEvent,
				wantCreated: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				tt.preInsert(t)

				created, err := repo.Insert(ctx, testEventID1, "faction_selected")
				require.NoError(t, err)
				assert.Equal(t, tt.wantCreated, created)
			})
		}
	})
}
