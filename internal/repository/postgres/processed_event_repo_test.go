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

func TestProcessedEventRepository_Insert(t *testing.T) {
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

func TestProcessedEventRepository_Insert_ParticipatesInTx(t *testing.T) {
	txMgr := postgres.NewTxManager(sharedPg.Pool)
	repo := postgres.NewProcessedEventRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)

	// tx 内で Insert → rollback すると行は残らないはず。
	err := txMgr.RunInTx(ctx, func(txCtx context.Context) error {
		created, ierr := repo.Insert(txCtx, testEventID1, "faction_selected")
		require.NoError(t, ierr)
		require.True(t, created)
		return assertRollback
	})
	require.ErrorIs(t, err, assertRollback)

	// rollback 後に再度 Insert できる = 行は残っていない。
	created, err := repo.Insert(ctx, testEventID1, "faction_selected")
	require.NoError(t, err)
	assert.True(t, created)
}

// assertRollback は TxManager の rollback テスト用のマーカーエラー。
var assertRollback = rollbackSentinel("intentional rollback")

type rollbackSentinel string

func (r rollbackSentinel) Error() string { return string(r) }
