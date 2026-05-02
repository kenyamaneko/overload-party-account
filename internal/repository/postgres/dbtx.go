// Package postgres は PostgreSQL による account サービスのデータアクセス実装を提供する。
//
// トランザクション方針: 全 repository メソッドは context 経由のトランザクション
// (TxManager.RunInTx が設定) に参加する。connFrom(ctx, pool) で透過的に Tx か Pool を選ぶ。
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.TxRunner = (*TxManager)(nil)

// dbtx は connFrom が pgxpool.Pool と pgx.Tx を透過的に切り替えるための最大公約数。
// pgx v5 の DBTX (sqlc 生成) と形が等価だが、本リポジトリは sqlc 未導入のため
// 自前定義する。package-private に閉じているのは、postgres パッケージ外からの
// 直接 DB アクセスを設計上禁じている (port 経由が SSoT) ため。
// sqlc を導入する場合は本 interface を捨てて sqlc 側の DBTX に寄せる。
type dbtx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

func txFromContext(ctx context.Context) dbtx {
	if tx, ok := ctx.Value(txKey{}).(dbtx); ok {
		return tx
	}
	return nil
}

func connFrom(ctx context.Context, pool *pgxpool.Pool) dbtx {
	if tx := txFromContext(ctx); tx != nil {
		return tx
	}
	return pool
}

// TxManager は pgxpool.Pool を使用した TxRunner の実装。
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager は TxManager を生成する。
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// RunInTx はトランザクション内で fn を実行する。
func (m *TxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
