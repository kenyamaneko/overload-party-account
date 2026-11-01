package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

const (
	uniqueViolationCode     = "23505"
	foreignKeyViolationCode = "23503"
)

var _ port.TxRunner = (*TxManager)(nil)

// dbtx は接続プールとトランザクションに共通する、SQL を実行する操作である。
type dbtx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// findTx は ctx に結び付いたトランザクションを返す。なければ nil を返す。
func findTx(ctx context.Context) dbtx {
	if tx, ok := ctx.Value(txKey{}).(dbtx); ok {
		return tx
	}
	return nil
}

// resolveDBTX は ctx にトランザクションがあればそれを、なければ接続プールを返す。
func resolveDBTX(ctx context.Context, pool *pgxpool.Pool) dbtx {
	if tx := findTx(ctx); tx != nil {
		return tx
	}
	return pool
}

// isUniqueViolation は err が一意制約違反かを返す。
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}

// isForeignKeyViolation は err が外部キー制約違反かを返す。
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolationCode
}

// TxManager は接続プールを使った port.TxRunner の PostgreSQL 実装である。
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager は TxManager を生成する。
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// RunInTx は fn を1つのトランザクションで実行する。
// fn が返したエラーと同じ原因のエラーを返し、その場合 fn の書き込みは確定しない。
// 呼び出しが取り消されていないのにトランザクションを開始できないときは、port.ErrDatabaseUnavailable を包んだエラーを返す。
func (m *TxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		return fmt.Errorf("begin tx: %w: %w", port.ErrDatabaseUnavailable, err)
	}
	defer func() { err = joinRollbackError(err, tx.Rollback(ctx)) }()

	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// joinRollbackError は、処理の結果 err に、巻き戻しの失敗 rollbackErr を合わせて返す。
func joinRollbackError(err, rollbackErr error) error {
	// 確定済みのトランザクションの巻き戻しは必ず ErrTxClosed になるため、失敗として扱わない。
	if rollbackErr == nil || errors.Is(rollbackErr, pgx.ErrTxClosed) {
		return err
	}
	return errors.Join(err, fmt.Errorf("rollback tx: %w", rollbackErr))
}
