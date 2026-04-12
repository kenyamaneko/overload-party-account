// Package repository は PostgreSQL によるデータアクセスを実装します。
//
// トランザクション方針:
// 全 repository メソッドは context 経由のトランザクション（TxManager.RunInTx が設定）
// に参加します。単文メソッドは connFrom(ctx, pool) で透過的にトランザクションまたは
// コネクションプールを使用します。複文メソッドは既存トランザクションを確認し、
// なければ独自トランザクションを開始します。
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.TxRunner = (*TxManager)(nil)

// dbtx は pgxpool.Pool と pgx.Tx の共通インターフェースです。
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

// TxManager は pgxpool.Pool を使用した TxRunner の実装です。
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager は TxManager を生成します。
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// RunInTx はトランザクション内で fn を実行します。
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

// MockTxRunner はテスト用の TxRunner 実装です。実際のトランザクションは開始しません。
type MockTxRunner struct{}

// RunInTx は fn を直接実行します（テスト用）。
func (m *MockTxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
