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

// Insert の契約: account.processed_events への INSERT が冪等性ガードとして機能する純プリミティブ。
// 初回挿入では (true, nil) を返し、event_id 重複時は ON CONFLICT DO NOTHING + RETURNING で
// (false, nil) を返す (Pub/Sub 重複配信の検出に使う)。
func TestInsert_ProcessedEvent(t *testing.T) {
	repo := postgres.NewProcessedEventRepository(sharedPg.Pool)
	ctx := context.Background()

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
			name:        "初回は created=true",
			preInsert:   noPreInsert,
			wantCreated: true,
		},
		{
			name:        "既存 event_id は created=false (冪等ガード)",
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
}

