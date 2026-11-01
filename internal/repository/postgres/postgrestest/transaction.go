package postgrestest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

const (
	lockWaitTimeout  = 10 * time.Second
	lockWaitInterval = 20 * time.Millisecond
)

// UncommittedUpdate は確定していないトランザクションの中で行った更新。
type UncommittedUpdate struct {
	release  func()
	finished chan error
}

// ReadResult は別のゴルーチンで行った読み取りの結果。
type ReadResult[T any] struct {
	Value T
	Err   error
}

// ReadInTx はトランザクションの中で読み取りを行い、読み取った値とエラーを返す。
func ReadInTx[T any](t *testing.T, txm *postgres.TxManager, read func(ctx context.Context) (T, error)) (T, error) {
	t.Helper()
	var value T
	err := txm.RunInTx(t.Context(), func(ctx context.Context) error {
		var readErr error
		value, readErr = read(ctx)
		return readErr
	})
	return value, err
}

// HoldUncommittedUpdate は別のトランザクションで更新を行い、確定させずに止めておく。
// ケースの終了時に、止めているトランザクションを解放する。
func HoldUncommittedUpdate(t *testing.T, txm *postgres.TxManager, update func(ctx context.Context) error) *UncommittedUpdate {
	t.Helper()
	updated := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- txm.RunInTx(context.Background(), func(ctx context.Context) error {
			err := update(ctx)
			close(updated)
			<-release
			return err
		})
	}()
	<-updated
	var once sync.Once
	releaseOnce := func() { once.Do(func() { close(release) }) }
	t.Cleanup(releaseOnce)
	return &UncommittedUpdate{release: releaseOnce, finished: finished}
}

// Commit は止めていたトランザクションを確定させ、確定に成功したことを確かめる。
func (u *UncommittedUpdate) Commit(t *testing.T) {
	t.Helper()
	u.release()
	require.NoError(t, <-u.finished)
}

// StartExclusiveRead は別のゴルーチンのトランザクションで読み取りを始め、結果を受け取るチャネルを返す。
func StartExclusiveRead[T any](txm *postgres.TxManager, read func(ctx context.Context) (T, error)) <-chan ReadResult[T] {
	results := make(chan ReadResult[T], 1)
	go func() {
		var result ReadResult[T]
		result.Err = txm.RunInTx(context.Background(), func(ctx context.Context) error {
			var readErr error
			result.Value, readErr = read(ctx)
			return readErr
		})
		results <- result
	}()
	return results
}

// RequireLockWaiter はケース専用のデータベースで、ロックの待ち状態に入った接続があることを確かめる。
func RequireLockWaiter(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var waiting int
		err := pool.QueryRow(t.Context(),
			"SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND datname = current_database()").Scan(&waiting)
		require.NoError(c, err)
		assert.Positive(c, waiting)
	}, lockWaitTimeout, lockWaitInterval)
}
